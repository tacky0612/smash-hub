#!/usr/bin/env bash
# SmashHub の本番インスタンスを操作するコマンド（docs/operations/deployment.md）。
# setup.sh が /usr/local/bin/smash-hub としてリンクを張る。root で実行する。
#
#   smash-hub start              起動する（イメージがなければビルドする）
#   smash-hub stop               停止する（データは消えない）
#   smash-hub restart            設定ファイルを読み直して起動し直す
#   smash-hub status             各サービスの状態を表示する
#   smash-hub logs [サービス]     ログを表示する（例: smash-hub logs backend）
#   smash-hub update [参照]       バックアップを取ってから、リポジトリを更新・再ビルド・再起動する
#   smash-hub backup             DB のバックアップを取る（毎日 systemd timer からも実行される）
#   smash-hub restore <ファイル>  バックアップから DB を復元する（今の DB の内容は消える）
#   smash-hub snapshot           ログのスナップショット（公開の対戦記録とレート）を書き出す
#   smash-hub check              設定ファイルを検証する
#
# 秘密情報（secrets.env の値）は標準出力・ログに出さない。
set -euo pipefail

SMASH_HUB_HOME="${SMASH_HUB_HOME:-/opt/smash-hub}"
export SMASH_HUB_HOME
REPO_DIR="$SMASH_HUB_HOME/repo"
CONFIG_FILE="$SMASH_HUB_HOME/smash-hub.env"
SECRETS_FILE="$SMASH_HUB_HOME/secrets.env"
BACKUP_DIR="$SMASH_HUB_HOME/backups"
SNAPSHOT_DIR="$SMASH_HUB_HOME/snapshots"
COMPOSE_FILE="$REPO_DIR/deploy/compose.yaml"

log() { printf '[smash-hub] %s\n' "$*"; }
die() { printf '[smash-hub] エラー: %s\n' "$*" >&2; exit 1; }

# 設定ファイルから KEY の値を読む（最後に書いた行が有効。前後の引用符は外す）
conf() {
	local value
	value="$(sed -n "s/^[[:space:]]*$1=//p" "$CONFIG_FILE" 2>/dev/null | tail -n 1)"
	value="${value%\"}"; value="${value#\"}"
	value="${value%\'}"; value="${value#\'}"
	printf '%s' "$value"
}

compose() {
	docker compose -f "$COMPOSE_FILE" --env-file "$CONFIG_FILE" --env-file "$SECRETS_FILE" "$@"
}

# 試用のドメイン（IP アドレスから作る無料のサブドメイン）か
is_trial_domain() {
	case "$1" in
		*.sslip.io | *.nip.io | *.traefik.me) return 0 ;;
		*) return 1 ;;
	esac
}

check() {
	[ -f "$CONFIG_FILE" ] || die "$CONFIG_FILE がありません。setup.sh を実行してください"
	[ -f "$SECRETS_FILE" ] || die "$SECRETS_FILE がありません。setup.sh を実行してください"
	[ -f "$COMPOSE_FILE" ] || die "$COMPOSE_FILE がありません。setup.sh を実行してください"

	local domain peers admins terms discord_id discord_secret
	domain="$(conf DOMAIN)"
	peers="$(conf PEERS)"
	admins="$(conf ADMIN_ACCOUNT_IDS)"
	terms="$(conf TERMS_FILE)"
	discord_id="$(conf DISCORD_CLIENT_ID)"
	discord_secret="$(conf DISCORD_CLIENT_SECRET)"

	[ -n "$domain" ] || die "$CONFIG_FILE の DOMAIN を設定してください"
	case "$domain" in
		*/* | *:*) die "DOMAIN にはドメイン名だけを書いてください（https:// やパスは不要です）: $domain" ;;
	esac
	# ADR 0009: IP アドレス由来のドメインの試用インスタンスは連合に参加させない
	if is_trial_domain "$domain" && [ -n "$peers" ]; then
		die "試用のドメイン（$domain）では連合に参加できません。独自ドメインに切り替えるか、PEERS を空にしてください"
	fi
	if [ -z "$discord_id" ] || [ -z "$discord_secret" ]; then
		log "注意: DISCORD_CLIENT_ID / DISCORD_CLIENT_SECRET が未設定です。設定するまで誰もログインできません（docs/operations/discord-app.md）"
	fi
	if [ -z "$admins" ]; then
		log "注意: ADMIN_ACCOUNT_IDS が未設定です。ログイン・登録したあと、自分のアカウント ID を設定してください"
	fi
	if [ -n "$terms" ]; then
		case "$terms" in
			/etc/smash-hub/custom/*)
				[ -f "$SMASH_HUB_HOME/custom/${terms#/etc/smash-hub/custom/}" ] ||
					die "TERMS_FILE のファイルがありません: $SMASH_HUB_HOME/custom/${terms#/etc/smash-hub/custom/}" ;;
			*) die "TERMS_FILE は /etc/smash-hub/custom/ の下を指定してください（ホストの $SMASH_HUB_HOME/custom/ に置いたファイル）" ;;
		esac
	fi
	compose config -q || die "Docker Compose の設定が正しくありません"
}

# 1GB の VPS でメモリが足りなくならないよう、1つずつビルドする
build() {
	log "イメージをビルドします（数分かかります）"
	compose build --pull backend
	compose build --pull web
	compose pull --quiet caddy db
}

start() {
	check
	if ! docker image inspect smash-hub-backend:local >/dev/null 2>&1 ||
		! docker image inspect smash-hub-web:local >/dev/null 2>&1; then
		build
	fi
	compose up -d --wait --wait-timeout 300
	log "起動しました: https://$(conf DOMAIN)/"
}

backup() {
	local keep stamp file tmp
	keep="$(conf BACKUP_KEEP)"
	keep="${keep:-14}"
	case "$keep" in '' | *[!0-9]*) die "BACKUP_KEEP は数字で指定してください" ;; esac
	[ "$keep" -ge 1 ] || die "BACKUP_KEEP は 1 以上にしてください"

	install -d -m 700 "$BACKUP_DIR"
	stamp="$(date +%Y%m%d-%H%M%S)"
	file="$BACKUP_DIR/smashhub-$stamp.dump"
	tmp="$file.partial"
	umask 077
	# アクセスログ（IP アドレスを含む。保持期間 14日）と OAuth の一時的な state はバックアップに含めない。
	# バックアップは保持期間を管理できないため（docs/security/data-protection.md）
	if ! compose exec -T db pg_dump -U smashhub -d smashhub --format=custom \
		--exclude-table-data=access_logs --exclude-table-data=oauth_states >"$tmp"; then
		rm -f -- "$tmp"
		die "バックアップに失敗しました（db が起動しているか確認してください）"
	fi
	mv "$tmp" "$file"
	log "バックアップを作成しました: $file"

	# 新しいものから keep 個を残して削除する
	find "$BACKUP_DIR" -maxdepth 1 -name 'smashhub-*.dump' -type f | sort -r | tail -n +"$((keep + 1))" |
		while read -r old; do
			rm -f -- "$old"
			log "古いバックアップを削除しました: $old"
		done
}

restore() {
	local file="${1:-}"
	[ -n "$file" ] || die "復元するバックアップのファイルを指定してください（例: smash-hub restore $BACKUP_DIR/smashhub-20260101-033000.dump）"
	[ -f "$file" ] || die "ファイルがありません: $file"
	check

	if [ "${SMASH_HUB_YES:-}" != "1" ]; then
		printf '今の DB の内容をすべて消して %s から復元します。よろしいですか？ [yes/no] ' "$file"
		local answer
		read -r answer
		[ "$answer" = "yes" ] || die "中止しました"
	fi

	log "backend・web・caddy を停止します"
	compose stop caddy web backend
	compose up -d --wait db
	log "DB を作り直して復元します"
	compose exec -T db psql -U smashhub -d postgres -v ON_ERROR_STOP=1 -q \
		-c 'DROP DATABASE IF EXISTS smashhub WITH (FORCE)' \
		-c 'CREATE DATABASE smashhub OWNER smashhub'
	compose exec -T db pg_restore -U smashhub -d smashhub --no-owner --exit-on-error <"$file"
	log "復元しました。起動します"
	start
}

# ログのスナップショットを書き出す（docs/operations/deployment.md の「9.4 ログのスナップショット」）。
# 中身は連合で公開しているログとレートだけなので、バックアップと違って人に渡してよい
snapshot() {
	local file tmp
	install -d -m 755 "$SNAPSHOT_DIR"
	file="$SNAPSHOT_DIR/smashhub-snapshot-$(date +%Y%m%d-%H%M%S).json"
	tmp="$file.partial"
	if ! compose exec -T backend /server snapshot >"$tmp"; then
		rm -f -- "$tmp"
		die "スナップショットの書き出しに失敗しました（backend が起動しているか確認してください）"
	fi
	mv "$tmp" "$file"
	chmod 644 "$file"
	log "スナップショットを作成しました: $file"
}

update() {
	local ref="${1:-}"
	ref="${ref:-$(conf SMASH_HUB_REF)}"
	ref="${ref:-main}"
	check

	if compose ps --status running --services 2>/dev/null | grep -qx db; then
		log "更新の前にバックアップを取ります"
		backup
	fi

	log "リポジトリを取得します（$ref）"
	git -C "$REPO_DIR" fetch --quiet --tags --force origin
	local target
	if git -C "$REPO_DIR" rev-parse --verify --quiet "refs/remotes/origin/$ref^{commit}" >/dev/null; then
		target="origin/$ref"
	elif git -C "$REPO_DIR" rev-parse --verify --quiet "refs/tags/$ref^{commit}" >/dev/null; then
		target="refs/tags/$ref"
	else
		die "ブランチまたはタグが見つかりません: $ref"
	fi
	git -C "$REPO_DIR" -c advice.detachedHead=false checkout --quiet --detach "$target"
	log "$(git -C "$REPO_DIR" log -1 --format='%h %s')"

	# 取得した新しい compose.yaml で検証し直す
	check
	build
	# DB のマイグレーションは backend が起動時に自動で実行する
	compose up -d --wait --wait-timeout 300 --remove-orphans
	docker image prune -f >/dev/null
	log "更新しました"
}

main() {
	[ "$(id -u)" -eq 0 ] || die "root で実行してください（sudo smash-hub ...）"
	local cmd="${1:-}"
	shift || true
	case "$cmd" in
		start) start ;;
		stop) compose stop ;;
		restart)
			check
			# 設定ファイル・Caddyfile の変更を反映するため、db 以外のコンテナを作り直す
			compose up -d --wait --wait-timeout 300 db
			compose up -d --wait --wait-timeout 300 --force-recreate backend web caddy
			;;
		status) compose ps ;;
		logs) compose logs --tail 200 -f "$@" ;;
		update) update "$@" ;;
		backup) backup ;;
		restore) restore "$@" ;;
		snapshot) snapshot ;;
		check) check && log "設定に問題はありません" ;;
		*)
			sed -n '2,14p' "$0" | sed 's/^# \{0,1\}//'
			[ -z "$cmd" ] || exit 1
			;;
	esac
}

# update でこのファイル自体が書き換わっても、読み込み済みの行だけを実行して終わるよう、同じ行で exit する
main "$@"; exit $?
