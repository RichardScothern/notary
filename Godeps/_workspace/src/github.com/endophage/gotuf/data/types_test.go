package data

import (
	"bytes"
	"encoding/json"

	. "gopkg.in/check.v1"
)

type TypesSuite struct{}

var _ = Suite(&TypesSuite{})

func (TypesSuite) TestGenerateFileMetaDefault(c *C) {
	// default is sha512
	r := bytes.NewReader([]byte("foo"))
	meta, err := NewFileMeta(r, "sha512")
	c.Assert(err, IsNil)
	c.Assert(meta.Length, Equals, int64(3))
	hashes := meta.Hashes
	c.Assert(hashes, HasLen, 1)
	hash, ok := hashes["sha512"]
	if !ok {
		c.Fatal("missing sha512 hash")
	}
	c.Assert(hash.String(), DeepEquals, "f7fbba6e0636f890e56fbbf3283e524c6fa3204ae298382d624741d0dc6638326e282c41be5e4254d8820772c5518a2c5a8c0c7f7eda19594a7eb539453e1ed7")
}

func (TypesSuite) TestGenerateFileMetaExplicit(c *C) {
	r := bytes.NewReader([]byte("foo"))
	meta, err := NewFileMeta(r, "sha256", "sha512")
	c.Assert(err, IsNil)
	c.Assert(meta.Length, Equals, int64(3))
	hashes := meta.Hashes
	c.Assert(hashes, HasLen, 2)
	for name, val := range map[string]string{
		"sha256": "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae",
		"sha512": "f7fbba6e0636f890e56fbbf3283e524c6fa3204ae298382d624741d0dc6638326e282c41be5e4254d8820772c5518a2c5a8c0c7f7eda19594a7eb539453e1ed7",
	} {
		hash, ok := hashes[name]
		if !ok {
			c.Fatalf("missing %s hash", name)
		}
		c.Assert(hash.String(), DeepEquals, val)
	}
}

func (TypesSuite) TestSignatureUnmarshalJSON(c *C) {
	signatureJSON := `{"keyid":"97e8e1b51b6e7cf8720a56b5334bd8692ac5b28233c590b89fab0b0cd93eeedc","method":"RSA","sig":"2230cba525e4f5f8fc744f234221ca9a92924da4cc5faf69a778848882fcf7a20dbb57296add87f600891f2569a9c36706314c240f9361c60fd36f5a915a0e9712fc437b761e8f480868d7a4444724daa0d29a2669c0edbd4046046649a506b3d711d0aa5e70cb9d09dec7381e7de27a3168e77731e08f6ed56fcce2478855e837816fb69aff53412477748cd198dce783850080d37aeb929ad0f81460ebd31e61b772b6c7aa56977c787d4281fa45dbdefbb38d449eb5bccb2702964a52c78811545939712c8280dee0b23b2fa9fbbdd6a0c42476689ace655eba0745b4a21ba108bcd03ad00fdefff416dc74e08486a0538f8fd24989e1b9fc89e675141b7c"}`

	var sig Signature
	err := json.Unmarshal([]byte(signatureJSON), &sig)
	c.Assert(err, IsNil)

	// Check that the method string is lowercased
	c.Assert(sig.Method.String(), Equals, "rsa")
}
