package helpers

func ShortenToken(token string) string {
	if len(token) >= 8 {
		return token[0:8]
	}
	return token
}

func ShortenTokenN(token string, length int) string {
	if len(token) >= length {
		return token[0:length]
	}
	return token
}
