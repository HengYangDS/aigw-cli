package secret

// TokenRecovery names the executable recovery owned by the selected secret
// backend and reports whether the backend can persist a replacement Token.
func TokenRecovery(store Store, account string) (instruction string, writable bool) {
	if IsReadOnly(store) {
		return "set environment variable " + EnvironmentKey(account), false
	}
	return "run `aigw rotate " + account + "`", true
}
