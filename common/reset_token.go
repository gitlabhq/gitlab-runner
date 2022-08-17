package common

func ResetToken(network Network, runner *RunnerCredentials, pat string) bool {
	var res *ResetTokenResponse
	if pat == "" {
		res = network.ResetToken(*runner)
	} else {
		res = network.ResetTokenWithPAT(*runner, pat)
	}

	if res == nil {
		return false
	}
	runner.Token = res.Token
	runner.TokenExpiresAt = res.TokenExpiresAt
	runner.TokenObtainedAt = res.TokenObtainedAt

	return true
}
