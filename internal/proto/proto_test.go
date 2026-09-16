package proto

import (
	"bytes"
	"io"
	"testing"
)

type pipe struct {
	io.Reader
	io.Writer
}

func (p pipe) Close() error { return nil }

func TestConnKeepsRawBytesAfterMessage(t *testing.T) {
	var buf bytes.Buffer
	out := NewConn(pipe{Reader: &buf, Writer: &buf})
	if err := out.Write(BlobHeader{OK: true, Size: 5, Hash: "abc"}); err != nil {
		t.Fatal(err)
	}
	if _, err := out.Raw().WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	if err := out.Raw().Flush(); err != nil {
		t.Fatal(err)
	}

	in := NewConn(pipe{Reader: &buf, Writer: io.Discard})
	var head BlobHeader
	if err := in.Read(&head); err != nil {
		t.Fatal(err)
	}
	if !head.OK || head.Size != 5 {
		t.Fatalf("header is %+v", head)
	}
	body := make([]byte, 5)
	if _, err := io.ReadFull(in.Bytes(), body); err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello" {
		t.Fatalf("body is %q", body)
	}
}

func TestProofNeedsTheRightKey(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	other := []byte("ffffffffffffffffffffffffffffffff")
	nonce := Nonce()
	proof := Proof(key, nonce, "folder1")

	if !CheckProof(key, nonce, "folder1", proof) {
		t.Fatal("the right key failed the check")
	}
	if CheckProof(other, nonce, "folder1", proof) {
		t.Fatal("another key passed the check")
	}
	if CheckProof(key, Nonce(), "folder1", proof) {
		t.Fatal("another challenge passed the check")
	}
	if CheckProof(key, nonce, "folder2", proof) {
		t.Fatal("another folder passed the check")
	}
}
