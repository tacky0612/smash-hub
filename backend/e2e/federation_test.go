//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Instance1 の利用者と Instance2 の利用者が、SmashHubプロトコルでインスタンスをまたいで対戦する一連の流れ。
//
//  1. taro（Instance1）が先に待ち、募集が Instance2 に配信される
//  2. hanako（Instance2）が探すと、配信された募集に申し込み、ホスト（Instance1）で対戦が成立する
//  3. taro が部屋 ID を入力し、hanako は Instance2 経由でそれを見る
//  4. 2人がそれぞれのインスタンスで報告し、hanako の報告はホストへ中継される
//  5. 対戦記録がホストのログに追記され、Instance2 が取り込み、両インスタンスで同じ戦績・レートになる
func TestCrossInstanceMatch(t *testing.T) {
	taro := newPlayer(t, instance1, "taro")
	hanako := newPlayer(t, instance2, "hanako")

	id := matchUntilBoth(t, taro, hanako)

	// 対戦ルーム: ホスト側の taro がプレイヤー1
	m := taro.match(id)
	if m.MyRole != "player1" || m.Player2.Player.ID != hanako.ID || m.RoomPassword != "1122" {
		t.Fatalf("ホスト側の対戦ルームが正しくない: %+v", m)
	}
	taro.call("PUT", "/api/matches/"+id+"/room", map[string]string{"roomId": "AB12C"}, http.StatusOK, nil)
	if m := hanako.match(id); m.MyRole != "player2" || m.RoomID != "AB12C" || m.Player1.Player.ID != taro.ID || m.Player2.Fighter != "fox" {
		t.Fatalf("Instance2 から見た対戦ルームが正しくない: %+v", m)
	}

	// 報告: hanako の報告（ゲームごとのファイターと、その署名を含む）は Instance2 から Instance1（ホスト）へ中継される
	g := games([2]string{"mario", "fox"}, [2]string{"mario", "pikachu"}, [2]string{"link", "pikachu"})
	taro.reportPicks(id, "win", g)
	if m := hanako.reportPicks(id, "lose", g); m.Status != "confirmed" || m.Winner != taro.ID || len(m.RecordedPicks) != 3 {
		t.Fatalf("報告が一致しても確定しない: %+v", m)
	}

	// 両インスタンスで同じ戦績・レートになる（Instance2 はログを取り込んでから反映）
	for _, base := range []string{instance1, instance2} {
		eventually(t, base+" のレート反映", func() bool {
			a, b := taro.standingOn(base, taro.ID), taro.standingOn(base, hanako.ID)
			return a.Rating == 1516 && a.Wins == 1 && b.Rating == 1484 && b.Losses == 1
		})
	}
	// 他インスタンスの利用者の表示名も、取り込んだ AccountPublished で表示できる
	eventually(t, "表示名の取り込み", func() bool {
		return taro.standingOn(instance1, hanako.ID).DisplayName == hanako.match(id).Player2.Player.DisplayName &&
			taro.standingOn(instance2, taro.ID).DisplayName != "（他インスタンスの利用者）"
	})

	// hanako の戦績（Instance2）にもこの対戦が載る
	eventually(t, "Instance2 の戦績", func() bool {
		var history []match
		hanako.call("GET", "/api/accounts/"+hanako.ID+"/matches", nil, http.StatusOK, &history)
		return len(history) > 0 && history[0].ID == id && history[0].Status == "confirmed" && history[0].Winner == taro.ID &&
			len(history[0].RecordedPicks) == 3
	})

	// 確定後は2人とも待機中でも対戦中でもない
	if a, b := taro.status(), hanako.status(); a.State != "idle" || b.State != "idle" {
		t.Errorf("確定後の状態: taro=%+v hanako=%+v", a, b)
	}
}

// 同じインスタンス内の対戦の記録も、連合先に複製される（Instance2 でホスト → Instance1 で取り込む）。
func TestLocalMatchIsReplicatedToPeer(t *testing.T) {
	jiro := newPlayer(t, instance2, "jiro")
	sachiko := newPlayer(t, instance2, "sachiko")

	// 同じインスタンスの2人は、ローカルでマッチングする
	id := matchUntilBoth(t, jiro, sachiko)
	jiro.report(id, "lose")
	if m := sachiko.report(id, "win"); m.Status != "confirmed" || m.Winner != sachiko.ID {
		t.Fatalf("確定しない: %+v", m)
	}

	eventually(t, "Instance1 への複製", func() bool {
		c, d := jiro.standingOn(instance1, jiro.ID), jiro.standingOn(instance1, sachiko.ID)
		return c.Rating == 1484 && c.Losses == 1 && d.Rating == 1516 && d.Wins == 1 && d.DisplayName != "（他インスタンスの利用者）"
	})
}

// 中止の報告がそろった対戦は記録されず、どちらのインスタンスでもレートは変わらない。
func TestCrossInstanceCancelIsNotRecorded(t *testing.T) {
	saburo := newPlayer(t, instance1, "saburo")
	yoshiko := newPlayer(t, instance2, "yoshiko")
	id := matchUntilBoth(t, saburo, yoshiko)
	saburo.report(id, "cancel")
	if m := yoshiko.report(id, "cancel"); m.Status != "cancelled" {
		t.Fatalf("中止にならない: %+v", m)
	}
	for _, base := range []string{instance1, instance2} {
		if s := saburo.standingOn(base, saburo.ID); s.Matches != 0 || s.Rating != 1500 {
			t.Errorf("%s: 中止でレートが変わった: %+v", base, s)
		}
	}
}

// 連合の API は、連合先のインスタンスの署名がないリクエストを拒否する。
func TestFederationAPIRequiresInstanceSignature(t *testing.T) {
	anon := &player{t: t, base: instance1, client: &http.Client{}, Name: "anonymous"}
	anon.call("POST", "/federation/match-claims", map[string]any{"request_id": "x"}, http.StatusForbidden, nil)
	anon.call("POST", "/federation/match-reports", map[string]any{"match_id": "x"}, http.StatusForbidden, nil)
	// ログとインスタンス情報は公開（誰でも検証できる）
	var info struct {
		InstanceID string `json:"instance_id"`
		PublicKey  string `json:"public_key"`
	}
	anon.call("GET", "/federation/instance", nil, http.StatusOK, &info)
	if info.InstanceID == "" || info.PublicKey == "" {
		t.Errorf("インスタンス情報が空: %+v", info)
	}
}

// インスタンスをまたぐ対戦で、ゲームごとのファイターが標準ルールに沿わない報告は中継の前に拒否し、
// 両者の入力がそろわなければ1戦目だけを記録する。その記録が連合先にも同じ内容で複製される。
func TestCrossInstancePicksAreValidatedAndFallBack(t *testing.T) {
	shiro := newPlayer(t, instance1, "shiro")
	kazuko := newPlayer(t, instance2, "kazuko")
	id := matchUntilBoth(t, shiro, kazuko) // shiro（mario）がプレイヤー1、kazuko（fox）がプレイヤー2

	// shiro の 2-0 なのに、shiro（1戦目の勝者）が2戦目にファイターを変えた内容は受け付けない
	bad := games([2]string{"mario", "fox"}, [2]string{"link", "fox"})
	kazuko.call("POST", "/api/matches/"+id+"/report", map[string]any{"result": "lose", "picks": bad}, http.StatusBadRequest, nil)
	if m := shiro.match(id); m.Player2.Reported {
		t.Fatal("拒否した報告がホストに中継された")
	}

	// shiro だけがファイターを入力し、kazuko は入力しない → 勝敗が一致すれば確定し、1戦目だけを記録する
	shiro.reportPicks(id, "win", games([2]string{"mario", "fox"}, [2]string{"mario", "pikachu"}))
	if m := kazuko.report(id, "lose"); m.Status != "confirmed" || len(m.RecordedPicks) != 1 {
		t.Fatalf("1戦目だけの記録で確定する: %+v", m)
	}
	eventually(t, "Instance2 の戦績", func() bool {
		var history []match
		kazuko.call("GET", "/api/accounts/"+kazuko.ID+"/matches", nil, http.StatusOK, &history)
		return len(history) > 0 && history[0].ID == id && history[0].Status == "confirmed" &&
			len(history[0].RecordedPicks) == 1 && history[0].RecordedPicks[0].Characters[1] == "fox"
	})
}

// ホスト（Instance2）の管理者が無効化した記録は、連合先（Instance1）でもレート計算から除外される。
func TestVoidIsReplicatedToPeer(t *testing.T) {
	shiro := newPlayer(t, instance2, "shiro")
	umeko := newPlayer(t, instance2, "umeko")
	id := matchUntilBoth(t, shiro, umeko)
	shiro.report(id, "win")
	if m := umeko.report(id, "lose"); m.Status != "confirmed" {
		t.Fatalf("確定しない: %+v", m)
	}
	eventually(t, "Instance1 への記録の複製", func() bool {
		return shiro.standingOn(instance1, shiro.ID).Matches == 1
	})

	// 連合先（Instance1）の管理者は、Instance2 がホストした対戦を無効化できない
	newAdmin(t, instance1).call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "collusion"}, http.StatusNotFound, nil)

	newAdmin(t, instance2).call("POST", "/api/admin/voids", map[string]string{"matchId": id, "reason": "collusion"}, http.StatusNoContent, nil)
	for _, p := range []*player{shiro, umeko} {
		if s := p.standingOn(instance2, p.ID); s.Matches != 0 || s.Rating != 1500 {
			t.Errorf("Instance2: 無効化後のレート %+v", s)
		}
	}
	eventually(t, "Instance1 への無効化の複製", func() bool {
		a, b := shiro.standingOn(instance1, shiro.ID), shiro.standingOn(instance1, umeko.ID)
		return a.Matches == 0 && a.Rating == 1500 && b.Matches == 0 && b.Rating == 1500
	})
}

// 利用停止中の利用者は、自分のインスタンスで対戦相手を探せない。
func TestSuspendedPlayerCannotSearch(t *testing.T) {
	goro := newPlayer(t, instance1, "goro")
	newAdmin(t, instance1).call("POST", "/api/admin/sanctions", map[string]any{"accountId": goro.ID, "kind": "red", "reason": "cheating"}, http.StatusCreated, nil)
	goro.call("POST", "/api/matchmaking/search", map[string]any{"fighter": "mario", "rateRange": 0}, http.StatusForbidden, nil)
}
