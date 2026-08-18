//go:build unit

package admin

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestOptionalProviderNotesDistinguishesOmittedAndClearedValues(t *testing.T) {
	var omitted UpdateProviderRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatalf("unmarshal omitted notes: %v", err)
	}
	if omitted.Notes.Set || omitted.Notes.Value != nil {
		t.Fatalf("omitted notes = %+v, want unset", omitted.Notes)
	}

	for name, payload := range map[string]string{
		"null":  `{"notes":null}`,
		"empty": `{"notes":""}`,
	} {
		t.Run(name, func(t *testing.T) {
			var request UpdateProviderRequest
			if err := json.Unmarshal([]byte(payload), &request); err != nil {
				t.Fatalf("unmarshal notes: %v", err)
			}
			if !request.Notes.Set || request.Notes.Value == nil || *request.Notes.Value != "" {
				t.Fatalf("notes = %+v, want explicit clear", request.Notes)
			}
		})
	}

	var populated UpdateProviderRequest
	if err := json.Unmarshal([]byte(`{"notes":"provider note"}`), &populated); err != nil {
		t.Fatalf("unmarshal populated notes: %v", err)
	}
	if !populated.Notes.Set || populated.Notes.Value == nil || *populated.Notes.Value != "provider note" {
		t.Fatalf("notes = %+v, want populated value", populated.Notes)
	}
}

func TestOptionalProviderProxyIDDistinguishesKeepDirectAndProxy(t *testing.T) {
	var omitted UpdateProviderRequest
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatalf("unmarshal omitted proxy: %v", err)
	}
	if omitted.ProxyID.Set || omitted.ProxyID.Value != nil {
		t.Fatalf("omitted proxy = %+v, want unset", omitted.ProxyID)
	}

	var direct UpdateProviderRequest
	if err := json.Unmarshal([]byte(`{"proxy_id":null}`), &direct); err != nil {
		t.Fatalf("unmarshal direct proxy: %v", err)
	}
	if !direct.ProxyID.Set || direct.ProxyID.Value != nil {
		t.Fatalf("direct proxy = %+v, want explicit nil", direct.ProxyID)
	}

	var proxied UpdateProviderRequest
	if err := json.Unmarshal([]byte(`{"proxy_id":7}`), &proxied); err != nil {
		t.Fatalf("unmarshal proxy ID: %v", err)
	}
	if !proxied.ProxyID.Set || proxied.ProxyID.Value == nil || *proxied.ProxyID.Value != 7 {
		t.Fatalf("proxied = %+v, want proxy 7", proxied.ProxyID)
	}
}

func TestParseProviderHealthOverviewIDs(t *testing.T) {
	ids, ok := parseProviderHealthOverviewIDs(" 3,1,3,2 ")
	if !ok || len(ids) != 3 || ids[0] != 3 || ids[1] != 1 || ids[2] != 2 {
		t.Fatalf("ids=%v ok=%v, want ordered unique IDs", ids, ok)
	}
	empty, ok := parseProviderHealthOverviewIDs("")
	if !ok || len(empty) != 0 {
		t.Fatalf("empty ids=%v ok=%v", empty, ok)
	}
	for _, invalid := range []string{"0", "-1", "1,nope", "1,,2"} {
		if _, ok := parseProviderHealthOverviewIDs(invalid); ok {
			t.Fatalf("input %q should be rejected", invalid)
		}
	}

	tooMany := make([]string, 101)
	for index := range tooMany {
		tooMany[index] = strconv.Itoa(index + 1)
	}
	if _, ok := parseProviderHealthOverviewIDs(strings.Join(tooMany, ",")); ok {
		t.Fatal("more than 100 IDs should be rejected")
	}
}

func TestParseProviderProbeHistoryQuery(t *testing.T) {
	limit, sinceSeconds, ok := parseProviderProbeHistoryQuery("", "")
	if !ok || limit != 100 || sinceSeconds != 3600 {
		t.Fatalf("defaults=(%d,%d,%v), want (100,3600,true)", limit, sinceSeconds, ok)
	}
	limit, sinceSeconds, ok = parseProviderProbeHistoryQuery("25", "7200")
	if !ok || limit != 25 || sinceSeconds != 7200 {
		t.Fatalf("parsed=(%d,%d,%v), want (25,7200,true)", limit, sinceSeconds, ok)
	}
	limit, sinceSeconds, ok = parseProviderProbeHistoryQuery("2000", "86400")
	if !ok || limit != 2000 || sinceSeconds != 86400 {
		t.Fatalf("24h boundary=(%d,%d,%v), want (2000,86400,true)", limit, sinceSeconds, ok)
	}
	for _, input := range [][2]string{{"0", ""}, {"2001", ""}, {"nope", ""}, {"", "59"}, {"", "86401"}, {"", "nope"}} {
		if _, _, valid := parseProviderProbeHistoryQuery(input[0], input[1]); valid {
			t.Fatalf("query %q/%q should be rejected", input[0], input[1])
		}
	}
}
