package encrypt

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
)

func MD5(data, secrete []byte) string {
	h := md5.New()
	h.Write(data)
	h.Write(secrete)
	res := h.Sum(nil)
	return hex.EncodeToString(res)
}

func SHA256(data, secrete []byte) string {
	h := sha256.New()
	h.Write(data)
	h.Write(secrete)
	res := h.Sum(nil)
	return hex.EncodeToString(res)
}
