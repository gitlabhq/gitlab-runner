package common

func ResetToken(network Network, runner *RunnerCredentials, systemID string, pat string) bool {
	var res *ResetTokenResponse
	if pat == "" {
		res = network.ResetToken(*runner, systemID)
	} else {
		res = network.ResetTokenWithPAT(*runner, systemID, pat)
	}

	if res == nil {
		return false
	}
	runner.Token = res.Token
	runner.TokenExpiresAt = res.TokenExpiresAt
	runner.TokenObtainedAt = res.TokenObtainedAt

	return true
}
