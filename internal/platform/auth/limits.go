package auth

const (
	maximumOIDCTokenBytes    = 16 << 10
	maximumOIDCMetadataBytes = 64 << 10
	maximumJWKSBytes         = 1 << 20
	minimumRSABits           = 2048
)
