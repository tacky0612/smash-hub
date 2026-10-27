//go:build integration

package integrationtest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/adapter"
	"github.com/tacky0612/smash-hub/backend/internal/modules/federation/domain"
	"github.com/tacky0612/smash-hub/backend/internal/platform/crypto"
)

// 対戦が確定すると、両者の署名付きの記録がインスタンス署名付きのログに追記され、
// /federation/log から取得した誰もがハッシュチェーンと署名を検証できる。
func TestFederationLog_ConfirmedMatchIsPublishedAndVerifiable(t *testing.T) {
	e := newEnv(t)
	taro, hanako := newPlayer(t, e, "taro"), newPlayer(t, e, "hanako")
	id := matchPair(t, taro, hanako)
	taro.report(id, "win")
	hanako.report(id, "lose")

	var info struct {
		InstanceID string `json:"instance_id"`
		PublicKey  string `json:"public_key"`
	}
	taro.call("GET", "/federation/instance", nil, http.StatusOK, &info)
	var log struct {
		Events []json.RawMessage `json:"events"`
	}
	taro.call("GET", "/federation/log?since=0", nil, http.StatusOK, &log)

	// taro・hanako の AccountPublished と、対戦の MatchRecorded の3件
	if len(log.Events) != 3 {
		t.Fatalf("ログの件数 = %d", len(log.Events))
	}
	verify := func(m []byte, s string) bool { return adapter.VerifyKey(info.PublicKey, m, s) }
	var lastSeq int64
	lastHash := ""
	var types []string
	for _, raw := range log.Events {
		var ev domain.Event
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		if err := domain.VerifyChain(ev, info.InstanceID, lastSeq, lastHash, verify); err != nil {
			t.Fatalf("seq %d: %v", ev.Seq, err)
		}
		switch ev.Type {
		case domain.TypeAccountPublished:
			if _, err := domain.DecodeAccountPublished(ev, crypto.VerifyDID); err != nil {
				t.Fatal(err)
			}
		case domain.TypeMatchRecorded:
			r, err := domain.DecodeMatchRecorded(ev, crypto.VerifyDID)
			if err != nil {
				t.Fatal(err)
			}
			if string(r.MatchID) != id || string(r.Players[r.Winner].AccountID) != taro.ID {
				t.Errorf("記録の内容が正しくない: %+v", r)
			}
		}
		types = append(types, ev.Type)
		lastSeq, lastHash = ev.Seq, ev.Hash()
	}
	if types[2] != domain.TypeMatchRecorded {
		t.Errorf("イベントの順番 = %v", types)
	}
	// アカウント ID は公開鍵から導出した did:key
	if _, err := crypto.PublicKeyFromDID(taro.ID); err != nil {
		t.Errorf("アカウント ID が did:key ではない: %s", taro.ID)
	}
}

// 連合先（許可リスト）以外からのインスタンス間リクエストは拒否する。
func TestFederationAPI_RejectsUnsignedRequests(t *testing.T) {
	e := newEnv(t)
	p := &player{t: t, env: e, client: newClient(t)}
	p.call("POST", "/federation/match-claims", map[string]any{"request_id": "x"}, http.StatusForbidden, nil)
	p.call("GET", "/federation/matches/x", nil, http.StatusForbidden, nil)
}
