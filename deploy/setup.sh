#!/usr/bin/env bash
# SmashHub の本番インスタンスのセットアップスクリプト（docs/operations/deployment.md、ADR 0009）。
#
# Ubuntu 24.04 LTS を前提とします。次のどちらでも使えます。
#   - VPS 事業者のスタートアップスクリプト / cloud-init のユーザーデータとして、そのまま貼り付ける（初回起動時に root で実行される）
#   - SSH で接続して手動で実行する: sudo bash setup.sh
#
# 何度実行しても同じ結果になります（冪等）。既にある設定ファイル・秘密情報・データは上書きしません。
# SmashHub 本体の更新は、このスクリプトではなく `smash-hub update` で行います。
#
# やること:
#   1. OS のセキュリティ更新の自動適用（unattended-upgrades）・タイムゾーン・スワップ・ファイアウォール（ufw）
#   2. Docker Engine と Compose plugin の導入（Docker の公式リポジトリから）
#   3. リポジトリの取得（/opt/smash-hub/repo）
#   4. 秘密情報のランダム生成（/opt/smash-hub/secrets.env）と、設定ファイルのひな形（/opt/smash-hub/smash-hub.env）
#   5. 毎日の DB のバックアップ（systemd timer）
#   6. 起動（DOMAIN が決まっている場合）
#
# 秘密情報は標準出力・ログに出しません（スタートアップスクリプトの出力は VPS のログに残るため）。
set -euo pipefail

# ---- 変数（環境変数で上書きできる。スタートアップスクリプトとして使う場合は書き換えずにそのまま使う） ----
# 取得するリポジトリと、ブランチまたはリリースタグ
SMASH_HUB_REPO="${SMASH_HUB_REPO:-https://github.com/tacky0612/smash-hub.git}"
SMASH_HUB_REF="${SMASH_HUB_REF:-main}"
# インストール先
SMASH_HUB_HOME="${SMASH_HUB_HOME:-/opt/smash-hub}"
# ドメイン（任意）。空なら、サーバーのグローバル IP アドレスから試用のドメイン（<IP>.sslip.io）を作る
SMASH_HUB_DOMAIN="${SMASH_HUB_DOMAIN:-}"
# スワップの大きさ（1GB の VPS でイメージをビルドするために必要）
SWAP_SIZE="${SWAP_SIZE:-2G}"
TIMEZONE="${TIMEZONE:-Asia/Tokyo}"
# OS の更新で再起動が必要になったときに自動で再起動する時刻（空なら自動で再起動しない）
AUTO_REBOOT_TIME="${AUTO_REBOOT_TIME:-04:30}"
# 毎日のバックアップの時刻（systemd の OnCalendar 形式）
BACKUP_TIME="${BACKUP_TIME:-*-*-* 03:30:00}"

export DEBIAN_FRONTEND=noninteractive
export HOME="${HOME:-/root}"

log() { printf '[smash-hub setup] %s\n' "$*"; }
die() { printf '[smash-hub setup] エラー: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "root で実行してください（sudo bash setup.sh）"
if [ -r /etc/os-release ]; then
	# shellcheck disable=SC1091
	. /etc/os-release
	[ "${ID:-}" = "ubuntu" ] || log "注意: Ubuntu 以外（${ID:-不明}）では動作を確認していません"
fi

apt_install() {
	apt-get install -y -q --no-install-recommends "$@" >/dev/null
}

# ---- 1. OS ----
setup_os() {
	log "OS のパッケージを準備します"
	apt-get update -q >/dev/null
	apt_install ca-certificates curl git openssl ufw unattended-upgrades tzdata

	if [ -n "$TIMEZONE" ] && [ -e "/usr/share/zoneinfo/$TIMEZONE" ]; then
		timedatectl set-timezone "$TIMEZONE" 2>/dev/null || ln -sf "/usr/share/zoneinfo/$TIMEZONE" /etc/localtime
	fi

	log "OS のセキュリティ更新を自動で適用するようにします"
	cat >/etc/apt/apt.conf.d/20auto-upgrades <<-'EOF'
		APT::Periodic::Update-Package-Lists "1";
		APT::Periodic::Unattended-Upgrade "1";
		APT::Periodic::AutocleanInterval "7";
	EOF
	if [ -n "$AUTO_REBOOT_TIME" ]; then
		cat >/etc/apt/apt.conf.d/52smash-hub-reboot <<-EOF
			// SmashHub: カーネル等の更新で再起動が必要になったら、利用の少ない時刻に自動で再起動する（コンテナは自動で起動し直す）
			Unattended-Upgrade::Automatic-Reboot "true";
			Unattended-Upgrade::Automatic-Reboot-Time "$AUTO_REBOOT_TIME";
		EOF
	fi
	systemctl enable --now unattended-upgrades >/dev/null 2>&1 || true

	if ! swapon --show=NAME --noheadings | grep -q .; then
		if [ ! -f /swapfile ]; then
			log "スワップ（$SWAP_SIZE）を作成します"
			fallocate -l "$SWAP_SIZE" /swapfile || dd if=/dev/zero of=/swapfile bs=1M count="$(numfmt --from=iec "$SWAP_SIZE" | awk '{print int($1/1048576)}')" status=none
			chmod 600 /swapfile
			mkswap /swapfile >/dev/null
		fi
		swapon /swapfile
		grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
	fi
	echo 'vm.swappiness=10' >/etc/sysctl.d/90-smash-hub.conf
	sysctl -q -p /etc/sysctl.d/90-smash-hub.conf || true

	log "ファイアウォール（ufw）で SSH・HTTP・HTTPS だけを許可します"
	ufw allow 22/tcp >/dev/null
	ufw allow 80/tcp >/dev/null
	ufw allow 443/tcp >/dev/null
	ufw allow 443/udp >/dev/null
	ufw --force enable >/dev/null
}

# ---- 2. Docker ----
setup_docker() {
	if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
		log "Docker は導入済みです"
	else
		log "Docker Engine と Compose plugin を Docker の公式リポジトリから導入します"
		install -m 0755 -d /etc/apt/keyrings
		curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
		chmod a+r /etc/apt/keyrings/docker.asc
		# shellcheck disable=SC1091
		codename="$(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")"
		cat >/etc/apt/sources.list.d/docker.sources <<-EOF
			Types: deb
			URIs: https://download.docker.com/linux/ubuntu
			Suites: $codename
			Components: stable
			Signed-By: /etc/apt/keyrings/docker.asc
		EOF
		apt-get update -q >/dev/null
		apt_install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
	fi
	systemctl enable --now docker >/dev/null
}

# ---- 3. リポジトリ ----
setup_repo() {
	install -d -m 755 "$SMASH_HUB_HOME"
	install -d -m 755 "$SMASH_HUB_HOME/custom"
	install -d -m 700 "$SMASH_HUB_HOME/backups"

	if [ -d "$SMASH_HUB_HOME/repo/.git" ]; then
		log "リポジトリは取得済みです（更新は smash-hub update で行います）"
	else
		log "リポジトリを取得します（$SMASH_HUB_REF）"
		git clone --quiet "$SMASH_HUB_REPO" "$SMASH_HUB_HOME/repo"
		git -C "$SMASH_HUB_HOME/repo" fetch --quiet --tags origin
		if git -C "$SMASH_HUB_HOME/repo" rev-parse --verify --quiet "refs/remotes/origin/$SMASH_HUB_REF^{commit}" >/dev/null; then
			target="origin/$SMASH_HUB_REF"
		else
			target="refs/tags/$SMASH_HUB_REF"
		fi
		git -C "$SMASH_HUB_HOME/repo" -c advice.detachedHead=false checkout --quiet --detach "$target"
	fi
	chmod 755 "$SMASH_HUB_HOME/repo/deploy/smash-hub.sh"
	ln -sf "$SMASH_HUB_HOME/repo/deploy/smash-hub.sh" /usr/local/bin/smash-hub
}

# ---- 4. 秘密情報と設定ファイル ----

# secrets.env に KEY がなければ、生成した値を書き足す（既にある値は変えない）。値は表示しない
add_secret() {
	local key="$1" value="$2"
	if ! grep -q "^$key=" "$SMASH_HUB_HOME/secrets.env"; then
		printf '%s=%s\n' "$key" "$value" >>"$SMASH_HUB_HOME/secrets.env"
		log "秘密情報 $key を生成しました"
	fi
}

setup_secrets() {
	local file="$SMASH_HUB_HOME/secrets.env"
	umask 077
	if [ ! -f "$file" ]; then
		cat >"$file" <<-'EOF'
			# SmashHub の秘密情報（setup.sh が生成。編集しない）
			# - インスタンスの署名鍵・識別子ペッパー・アカウント鍵の暗号化鍵・DB のパスワード
			# - 失うと連合でのインスタンスの同一性や、利用者のログインを失います。DB のバックアップとは別の安全な場所にも保管してください
			# - 漏えいした場合は docs/security/incident-response.md に従ってください
		EOF
	fi
	chmod 600 "$file"
	chown root:root "$file"
	# Ed25519 のシード（32 バイト、base64）
	add_secret INSTANCE_PRIVATE_KEY "$(openssl rand -base64 32)"
	# 32 バイト以上の値（hex で 64 文字）
	add_secret IDENTIFIER_PEPPER "$(openssl rand -hex 32)"
	add_secret ACCOUNT_KEY_SECRET "$(openssl rand -hex 32)"
	# DATABASE_URL に入れるため、URL で特別な意味を持つ文字を含まない hex にする
	add_secret DB_PASSWORD "$(openssl rand -hex 24)"
	umask 022
}

# サーバーのグローバル IPv4 アドレス（取れなければ空）
public_ipv4() {
	local ip
	ip="$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for (i = 1; i < NF; i++) if ($i == "src") print $(i + 1)}' | head -n 1)"
	case "$ip" in
		'' | 10.* | 192.168.* | 127.* | 169.254.* | 100.6[4-9].* | 100.[7-9][0-9].* | 100.1[01][0-9].* | 100.12[0-7].*) return 0 ;;
		172.1[6-9].* | 172.2[0-9].* | 172.3[01].*) return 0 ;;
	esac
	printf '%s' "$ip"
}

setup_config() {
	local file="$SMASH_HUB_HOME/smash-hub.env"
	if [ -f "$file" ]; then
		log "設定ファイルは作成済みです: $file"
		return
	fi
	local domain="$SMASH_HUB_DOMAIN" ip
	if [ -z "$domain" ]; then
		ip="$(public_ipv4)"
		[ -z "$ip" ] || domain="${ip//./-}.sslip.io"
	fi
	umask 077
	cat >"$file" <<-EOF
		# SmashHub の設定ファイル（運営者が編集する）。変更したら \`smash-hub restart\` で反映します。
		# 手順: docs/operations/deployment.md
		# 秘密情報（署名鍵など）はここではなく secrets.env にあります。

		# インスタンスのドメイン（https:// は付けない）。DNS の A レコードをこのサーバーの IP アドレスに向けてください。
		# 初期値の <IP アドレス>.sslip.io は試用のドメインです。試用のインスタンスは連合に参加できません（PEERS を設定できません）。
		# 連合で使うインスタンスの URL は変えられないため、本番では最初から独自ドメインにしてください。
		DOMAIN=$domain

		# インスタンス名（画面に表示される）
		INSTANCE_NAME="SmashHub"

		# Discord アプリのクライアント ID とクライアントシークレット（docs/operations/discord-app.md）
		# リダイレクト URI には https://<DOMAIN>/api/auth/discord/callback を登録します
		DISCORD_CLIENT_ID=
		DISCORD_CLIENT_SECRET=

		# 管理者のアカウント ID（did:key:...）。複数ならカンマ区切り。
		# 一度ログイン・登録してから自分のアカウント ID を書き、smash-hub restart します
		ADMIN_ACCOUNT_IDS=

		# 運営者への問い合わせ先。利用規約と画面に表示され、公開されます
		# 例: Discord サーバーの招待リンク、問い合わせフォームのリンク
		OPERATOR_CONTACT=

		# 連合先のインスタンスの URL（カンマ区切り）。例: PEERS=https://smash-hub.example.org,https://other.example.net
		PEERS=

		# インスタンス独自の利用規約（任意）。$SMASH_HUB_HOME/custom/terms.md を置いて、次の行のコメントを外す。
		# 設定しなければプロジェクト共通のひな形を表示します
		#TERMS_FILE=/etc/smash-hub/custom/terms.md

		# 改変したソースコードで運営する場合は、その公開先（AGPL-3.0 第13条）
		#SOURCE_CODE_URL=https://github.com/<あなたのアカウント>/smash-hub

		# DB のバックアップを残す数（毎日1つ作る。初期値 14）
		BACKUP_KEEP=14

		# smash-hub update で取得するブランチまたはリリースタグ
		SMASH_HUB_REF=$SMASH_HUB_REF
	EOF
	chmod 600 "$file"
	umask 022
	log "設定ファイルのひな形を作成しました: $file"
}

# ---- 5. バックアップ ----
setup_backup_timer() {
	log "毎日の DB のバックアップを設定します"
	cat >/etc/systemd/system/smash-hub-backup.service <<-EOF
		[Unit]
		Description=SmashHub の DB のバックアップ
		After=docker.service
		Requires=docker.service

		[Service]
		Type=oneshot
		Environment=SMASH_HUB_HOME=$SMASH_HUB_HOME
		ExecStart=/usr/local/bin/smash-hub backup
	EOF
	cat >/etc/systemd/system/smash-hub-backup.timer <<-EOF
		[Unit]
		Description=SmashHub の DB のバックアップ（毎日）

		[Timer]
		OnCalendar=$BACKUP_TIME
		RandomizedDelaySec=15min
		Persistent=true

		[Install]
		WantedBy=timers.target
	EOF
	systemctl daemon-reload
	systemctl enable --now smash-hub-backup.timer >/dev/null 2>&1
}

# ---- 6. 起動 ----
start_instance() {
	local domain
	domain="$(sed -n 's/^DOMAIN=//p' "$SMASH_HUB_HOME/smash-hub.env" | tail -n 1)"
	if [ -z "$domain" ]; then
		log "DOMAIN が未設定のため、まだ起動しません。$SMASH_HUB_HOME/smash-hub.env を編集してから smash-hub start を実行してください"
		return
	fi
	log "SmashHub を起動します（初回はイメージのビルドに 10分ほどかかります）"
	SMASH_HUB_HOME="$SMASH_HUB_HOME" /usr/local/bin/smash-hub start
}

main() {
	setup_os
	setup_docker
	setup_repo
	setup_secrets
	setup_config
	setup_backup_timer
	start_instance
	log "完了しました。設定ファイル: $SMASH_HUB_HOME/smash-hub.env（手順: docs/operations/deployment.md）"
}

main "$@"
