package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"

	"github.com/pydio/services/common/log"
	"github.com/pydio/services/common/proto/tree"
	"github.com/pydio/services/data/encryption/ciopher"
	"github.com/pydio/services/data/encryption/dao"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

type KeyManagerHandler struct {
	Dao                *dao.DAO
	NodeReceiverClient tree.NodeReceiverClient
	NodeProviderClient tree.NodeProviderClient
}

const (
	META_ENCRYPTION_KEY = "ENCRYPTION_KEY"
)

func (e *KeyManagerHandler) GetEncryptionKey(c context.Context, r *tree.GetEncryptionKeyRequest, w *tree.GetEncryptionKeyResponse) error {

	d := *e.Dao
	userWorkingKey, err := d.UserWorkingKey(r.User, r.Password)
	if err != nil {
		return err
	}

	log.Logger(c).Debug("Let's check the NODE !! ", zap.Any("node", r.Node))

	var strKey string
	r.Node.GetMeta(META_ENCRYPTION_KEY, &strKey)
	if strKey == "" {
		resp, err := e.NodeProviderClient.ReadNode(c, &tree.ReadNodeRequest{
			Node: r.Node,
		})
		if err != nil {
			return err
		}
		resp.Node.GetMeta(META_ENCRYPTION_KEY, &strKey)
	}

	log.Logger(c).Debug("Ok do we have a key?", zap.Int("length", len(strKey)))

	if len(strKey) == 0 && r.Create {
		key, err := GenerateEncryptionKey(32)
		if err != nil {
			return err
		}
		strKey = string(key)

		log.Logger(c).Debug("We have Enc Key", zap.Int("length", len(userWorkingKey)))

		ioCipher, err := ciopher.NewAESCipher(userWorkingKey)
		if err != nil {
			return err
		}

		log.Logger(c).Debug("After NewAES")

		buff := bytes.NewBuffer([]byte{})
		if err := ioCipher.Encrypt(bytes.NewBuffer([]byte(strKey)), buff); err != nil {
			return err
		}

		encoded := base64.StdEncoding.EncodeToString(buff.Bytes())
		r.Node.SetMeta(META_ENCRYPTION_KEY, encoded)
		e.NodeReceiverClient.UpdateNode(c, &tree.UpdateNodeRequest{
			From: r.Node,
			To:   r.Node,
		})

		log.Logger(c).Debug("Saved")

		w.Key = key

	} else if len(strKey) > 0 {
		decoded, _ := base64.StdEncoding.DecodeString(strKey)
		ioCipher, err := ciopher.NewAESCipher(userWorkingKey)
		if err != nil {
			return err
		}

		buff := bytes.NewBuffer([]byte{})
		if err := ioCipher.Decrypt(bytes.NewBuffer(decoded), buff); err != nil {
			return err
		}
		w.Key = buff.Bytes()
	}
	return nil
}

func GenerateEncryptionKey(size uint32) ([]byte, error) {
	if size != 16 && size != 24 && size != 32 {
		return nil, ciopher.Error("[GenerateEncryptionKey] Invalid key size")
	}
	key := make([]byte, size)
	_, err := rand.Read(key)
	return key, err
}
