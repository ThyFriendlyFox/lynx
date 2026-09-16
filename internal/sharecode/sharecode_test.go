package sharecode

import (
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	key, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	code := Encode(key)
	if !strings.HasPrefix(code, Prefix) {
		t.Fatalf("code %q has no prefix", code)
	}
	got, err := Decode(code)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(key) {
		t.Fatal("decoded key differs from the original key")
	}
}

func TestDecodeIsForgiving(t *testing.T) {
	key, _ := NewKey()
	code := Encode(key)
	messy := " " + strings.ToUpper(code[:20]) + "-" + code[20:] + "\n"
	got, err := Decode(messy)
	if err != nil {
		t.Fatalf("decode messy code: %v", err)
	}
	if string(got) != string(key) {
		t.Fatal("decoded key differs from the original key")
	}
}

func TestDecodeRejectsDamagedCode(t *testing.T) {
	key, _ := NewKey()
	code := []byte(Encode(key))
	if code[10] == 'a' {
		code[10] = 'b'
	} else {
		code[10] = 'a'
	}
	if _, err := Decode(string(code)); err == nil {
		t.Fatal("damaged code was accepted")
	}
	for _, bad := range []string{"", "lynx-", "hello", "lynx-!!!!"} {
		if _, err := Decode(bad); err == nil {
			t.Fatalf("bad code %q was accepted", bad)
		}
	}
}
