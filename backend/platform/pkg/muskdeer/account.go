package muskdeer

import (
	"strconv"
	"strings"
)

const AccountScopePrefix = "himalaya/"

// TradingID preserves the full positive int64 identifier supplied as decimal text.
func TradingID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0 && strconv.FormatInt(id, 10) == value
}

func AccountScope(user, fund int64) string {
	return UserScopePrefix(user) + strconv.FormatInt(fund, 10)
}

func UserScopePrefix(user int64) string {
	return AccountScopePrefix + strconv.FormatInt(user, 10) + "/"
}

func AccountUser(account string) (int64, bool) {
	parts := strings.Split(account, "/")
	if len(parts) != 3 || parts[0]+"/" != AccountScopePrefix {
		return 0, false
	}
	user, uok := TradingID(parts[1])
	_, fok := TradingID(parts[2])
	return user, uok && fok
}
