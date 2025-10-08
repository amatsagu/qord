package qord

import "time"

const (
	ADDON_VERSION = "v0.1"
	USER_AGENT    = "DiscordBot (https://github.com/amatsagu/qord, " + ADDON_VERSION + ")"
)

const (
	message_rate       = 5
	message_regen_time = time.Second * 5
	target_rate        = 10 // There's issue because it's per number of guilds bot is in, we'll figure it out maybe in future :)
	target_regen_time  = time.Second
)
