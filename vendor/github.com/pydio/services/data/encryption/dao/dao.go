package dao

import "crypto/rsa"


type DAO interface {
	UserPublicKey(user string) (*rsa.PublicKey, error)
	UserPrivateKey(user string, password string) (*rsa.PrivateKey, error)
	UserWorkingKey(user string, password string) ([]byte, error)
}

