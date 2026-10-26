//go:build e2e

package e2e

import (
	"net/http"
	"testing"
)

// Instance1 の利用者と Instance2 の利用者が、連合プロトコルでインスタンスをまたいで対戦する一連の流れ。
//
//  1. alice（Instance1）が先に待ち、募集が Instance2 に配信される
//  2. bob（Instance2）が探すと、配信された募集に申し込み、ホスト（Instance1）で対戦が成立する
//  3. alice が部屋 ID を入力し、bob は Instance2 経由でそれを見る
//  4. 2人がそれぞれのインスタンスで報告し、bob の報告はホストへ中継される
//  5. 対戦記録がホストのログに追記され、Instance2 が取り込み、両インスタンスで同じ戦績・レートになる
func TestCrossInstanceMatch(t *testing.T) {
	alice := newPlayer(t, instance1, "alice")
	bob := newPlayer(t, instance2, "bob")

	id := matchUntilBoth(t, alice, bob)

	// 対戦ルーム: ホスト側の alice がプレイヤー1
	m := alice.match(id)
	if m.MyRole != "player1" || m.Player2.Player.ID != bob.ID || m.RoomPassword != "1122" {
		t.Fatalf("ホスト側の対戦ルームが正しくない: %+v", m)
	}
	alice.call("PUT", "/api/matches/"+id+"/room", map[string]string{"roomId": "AB12C"}, http.StatusOK, nil)
	if m := bob.match(id); m.MyRole != "player2" || m.RoomID != "AB12C" || m.Player1.Player.ID != alice.ID || m.Player2.Fighter != "fox" {
		t.Fatalf("Instance2 から見た対戦ルームが正しくない: %+v", m)
	}

	// 報告: bob の報告は Instance2 から Instance1（ホスト）へ中継される
	alice.report(id, "win")
	if m := bob.report(id, "lose"); m.Status != "confirmed" || m.Winner != alice.ID {
		t.Fatalf("報告が一致しても確定しない: %+v", m)
	}

	// 両インスタンスで同じ戦績・レートになる（Instance2 はログを取り込んでから反映）
	for _, base := range []string{instance1, instance2} {
		eventually(t, base+" のレート反映", func() bool {
			a, b := alice.standingOn(base, alice.ID), alice.standingOn(base, bob.ID)
			return a.Rating == 1516 && a.Wins == 1 && b.Rating == 1484 && b.Losses == 1
		})
	}
	// 他インスタンスの利用者の表示名も、取り込んだ AccountPublished で表示できる
	eventually(t, "表示名の取り込み", func() bool {
		return alice.standingOn(instance1, bob.ID).DisplayName == bob.match(id).Player2.Player.DisplayName &&
			alice.standingOn(instance2, alice.ID).DisplayName != "（他インスタンスの利用者）"
	})

	// bob の戦績（Instance2）にもこの対戦が載る
	eventually(t, "Instance2 の戦績", func() bool {
		var history []match
		bob.call("GET", "/api/accounts/"+bob.ID+"/matches", nil, http.StatusOK, &history)
		return len(history) > 0 && history[0].ID == id && history[0].Status == "confirmed" && history[0].Winner == alice.ID
	})

	// 確定後は2人とも待機中でも対戦中でもない
	if a, b := alice.status(), bob.status(); a.State != "idle" || b.State != "idle" {
		t.Errorf("確定後の状態: alice=%+v bob=%+v", a, b)
	}
}

// 同じインスタンス内の対戦の記録も、連合先に複製される（Instance2 でホスト → Instance1 で取り込む）。
func TestLocalMatchIsReplicatedToPeer(t *testing.T) {
	carol := newPlayer(t, instance2, "carol")
	dave := newPlayer(t, instance2, "dave")

	// 同じインスタンスの2人は、ローカルでマッチングする
	id := matchUntilBoth(t, carol, dave)
	carol.report(id, "lose")
	if m := dave.report(id, "win"); m.Status != "confirmed" || m.Winner != dave.ID {
		t.Fatalf("確定しない: %+v", m)
	}

	eventually(t, "Instance1 への複製", func() bool {
		c, d := carol.standingOn(instance1, carol.ID), carol.standingOn(instance1, dave.ID)
		return c.Rating == 1484 && c.Losses == 1 && d.Rating == 1516 && d.Wins == 1 && d.DisplayName != "（他インスタンスの利用者）"
	})
}

// 中止の報告がそろった対戦は記録されず、どちらのインスタンスでもレートは変わらない。
func TestCrossInstanceCancelIsNotRecorded(t *testing.T) {
	erin := newPlayer(t, instance1, "erin")
	frank := newPlayer(t, instance2, "frank")
	id := matchUntilBoth(t, erin, frank)
	erin.report(id, "cancel")
	if m := frank.report(id, "cancel"); m.Status != "cancelled" {
		t.Fatalf("中止にならない: %+v", m)
	}
	for _, base := range []string{instance1, instance2} {
		if s := erin.standingOn(base, erin.ID); s.Matches != 0 || s.Rating != 1500 {
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
