package qord

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/amatsagu/tempest"
)

type Client struct {
	ApplicationID               tempest.Snowflake
	Gateway                     *ShardManager
	Rest                        *tempest.Rest
	RateLimiter                 *RateLimiter // Used only if Qord Client is set to respect rate limits - otherwise it will be nil pointer.
	traceLogger                 *log.Logger
	respectRateLimits           bool
	customEventHandler          func(shardID uint16, packet EventPacket)
	messageHandler              func(msg Message)
	commandInteractionHandler   func(itx CommandInteraction)
	componentInteractionHandler func(itx ComponentInteraction)
	modalInteractionHandler     func(itx ModalInteraction)
}

type ClientOptions struct {
	Token string
	// Client has own event handler for dealing with interactions but you can
	// still attach your own logic. It will be used before Client's default handler.
	EventHandler func(shardID uint16, packet EventPacket)
	// Attach own handler to read incoming messages.
	// It'll work properly only if bot has enabled special message content intent.
	MessageHandler func(msg Message)
	// Attach own handler to properly process all slash commands,
	// user commands, message commands or auto complete commands.
	CommandInteractionHandler func(itx CommandInteraction)
	// Attach own handler to process button clicks, menu selects...
	ComponentInteractionHandler func(itx ComponentInteraction)
	// Attach own handler to process modals.
	ModalInteractionHandler func(itx ModalInteraction)
	// Whether to try launching bot with special message content intent.
	// Remember to define MessageHandler to read incoming message packets.
	AcceptMessageContent bool
	// Whether basic Client's methods that use Tempest's Rest should automatically sleep when rate limited.
	// It uses naive rate limiter implementation that is not deeply synced with Rest logic itself, use with care.
	RespectRateLimits bool
	// Whether to have ShardManager print debug logs.
	Trace bool
}

func NewClient(opt ClientOptions) *Client {
	var botUserID tempest.Snowflake
	{
		strs := strings.Split(opt.Token, ".")
		if len(strs) == 0 {
			panic("token is not in a valid format")
		}

		hexID := strings.Replace(strs[0], "Bot ", "", 1)

		byteID, err := base64.RawStdEncoding.DecodeString(hexID)
		if err != nil {
			panic("failed to extract bot user ID from bot token: " + err.Error())
		}

		botUserID, err = tempest.StringToSnowflake(string(byteID))
		if err != nil {
			panic("failed to extract bot user ID from bot token: " + err.Error())
		}
	}

	self := Client{
		ApplicationID:               botUserID,
		Rest:                        tempest.NewRest(opt.Token),
		traceLogger:                 log.New(io.Discard, "[QORD] ", log.LstdFlags),
		customEventHandler:          opt.EventHandler,
		messageHandler:              undefinedMessageHandler,
		commandInteractionHandler:   undefinedCommandHandler,
		componentInteractionHandler: undefinedComponentHandler,
		modalInteractionHandler:     undefinedModalHandler,
	}
	self.Gateway = NewShardManager(opt.Token, opt.Trace, self.eventHandler)

	if opt.MessageHandler != nil {
		self.messageHandler = opt.MessageHandler
	}

	if opt.CommandInteractionHandler != nil {
		self.commandInteractionHandler = opt.CommandInteractionHandler
	}

	if opt.ComponentInteractionHandler != nil {
		self.componentInteractionHandler = opt.ComponentInteractionHandler
	}

	if opt.ModalInteractionHandler != nil {
		self.modalInteractionHandler = opt.ModalInteractionHandler
	}

	if opt.RespectRateLimits {
		self.RateLimiter = NewRateLimiter()
		self.respectRateLimits = true
	}

	if opt.Trace {
		self.traceLogger.SetOutput(os.Stdout)
		self.tracef("Main client tracing enabled.")
	}
	return &self
}

func (client *Client) SendMessage(channelID tempest.Snowflake, message tempest.Message, files []tempest.File) (tempest.Message, error) {
	if client.respectRateLimits {
		client.RateLimiter.WaitOrSet(channelID, message_rate, message_regen_time)
	}

	raw, err := client.Rest.RequestWithFiles(http.MethodPost, "/channels/"+channelID.String()+"/messages", message, files)
	if err != nil {
		return tempest.Message{}, err
	}

	res := tempest.Message{}
	err = json.Unmarshal(raw, &res)
	if err != nil {
		return tempest.Message{}, errors.New("failed to parse received data from discord")
	}

	return res, nil
}

func (client *Client) SendLinearMessage(channelID tempest.Snowflake, content string) (tempest.Message, error) {
	return client.SendMessage(channelID, tempest.Message{Content: content}, nil)
}

// Creates (or fetches if already exists) user's private text channel (DM) and tries to send message into it.
// Warning! Discord's user channels endpoint has huge rate limits so please reuse tempest.Message#ChannelID whenever possible.
func (client *Client) SendPrivateMessage(userID tempest.Snowflake, content tempest.Message, files []tempest.File) (tempest.Message, error) {
	if client.respectRateLimits {
		client.RateLimiter.WaitOrSet(userID, message_rate, message_regen_time)
	}

	res := make(map[string]any, 0)
	res["recipient_id"] = userID

	raw, err := client.Rest.Request(http.MethodPost, "/users/@me/channels", res)
	if err != nil {
		return tempest.Message{}, err
	}

	err = json.Unmarshal(raw, &res)
	if err != nil {
		return tempest.Message{}, errors.New("failed to parse received data from discord")
	}

	channelID, err := tempest.StringToSnowflake(res["id"].(string))
	if err != nil {
		return tempest.Message{}, err
	}

	msg, err := client.SendMessage(channelID, content, files)
	msg.ChannelID = channelID // Just in case.

	return msg, err
}

func (client *Client) EditMessage(channelID tempest.Snowflake, messageID tempest.Snowflake, content tempest.Message) error {
	if client.respectRateLimits {
		client.RateLimiter.WaitOrSet(channelID, message_rate, message_regen_time)
	}

	_, err := client.Rest.Request(http.MethodPatch, "/channels/"+channelID.String()+"/messages/"+messageID.String(), content)
	return err
}

func (client *Client) DeleteMessage(channelID tempest.Snowflake, messageID tempest.Snowflake) error {
	if client.respectRateLimits {
		client.RateLimiter.WaitOrSet(channelID, message_rate, message_regen_time)
	}

	_, err := client.Rest.Request(http.MethodDelete, "/channels/"+channelID.String()+"/messages/"+messageID.String(), nil)
	return err
}

func (client *Client) CrosspostMessage(channelID tempest.Snowflake, messageID tempest.Snowflake) error {
	if client.respectRateLimits {
		client.RateLimiter.WaitOrSet(channelID, message_rate, message_regen_time)
	}

	_, err := client.Rest.Request(http.MethodPost, "/channels/"+channelID.String()+"/messages/"+messageID.String()+"/crosspost", nil)
	return err
}

func (client *Client) FetchUser(id tempest.Snowflake) (tempest.User, error) {
	if client.respectRateLimits { // Put cooldown on self
		client.RateLimiter.WaitOrSet(client.ApplicationID, target_rate, target_regen_time)
	}

	raw, err := client.Rest.Request(http.MethodGet, "/users/"+id.String(), nil)
	if err != nil {
		return tempest.User{}, err
	}

	res := tempest.User{}
	err = json.Unmarshal(raw, &res)
	if err != nil {
		return tempest.User{}, errors.New("failed to parse received data from discord")
	}

	return res, nil
}

func (client *Client) FetchMember(guildID tempest.Snowflake, memberID tempest.Snowflake) (tempest.Member, error) {
	if client.respectRateLimits {
		client.RateLimiter.WaitOrSet(guildID, target_rate, target_regen_time)
	}

	raw, err := client.Rest.Request(http.MethodGet, "/guilds/"+guildID.String()+"/members/"+memberID.String(), nil)
	if err != nil {
		return tempest.Member{}, err
	}

	res := tempest.Member{}
	err = json.Unmarshal(raw, &res)
	if err != nil {
		return tempest.Member{}, errors.New("failed to parse received data from discord")
	}

	return res, nil
}

// Returns all entitlements for a given app, active and expired.
//
// By default it will attempt to return all, existing entitlements - provide query filter to control this behavior.
//
// https://discord.com/developers/docs/resources/entitlement#list-entitlements
func (client *Client) FetchEntitlementsPage(queryFilter string) ([]tempest.Entitlement, error) {
	if queryFilter[0] != '?' {
		queryFilter = "?" + queryFilter
	}

	res := make([]tempest.Entitlement, 0)
	raw, err := client.Rest.Request(http.MethodGet, "/applications/"+client.ApplicationID.String()+"/entitlements"+queryFilter, nil)
	if err != nil {
		return res, err
	}

	err = json.Unmarshal(raw, &res)
	if err != nil {
		return res, errors.New("failed to parse received data from discord")
	}

	return res, nil
}

// https://discord.com/developers/docs/resources/entitlement#get-entitlement
func (client *Client) FetchEntitlement(entitlementID tempest.Snowflake) (tempest.Entitlement, error) {
	raw, err := client.Rest.Request(http.MethodGet, "/applications/"+client.ApplicationID.String()+"/entitlements/"+entitlementID.String(), nil)
	if err != nil {
		return tempest.Entitlement{}, err
	}

	res := tempest.Entitlement{}
	err = json.Unmarshal(raw, &res)
	if err != nil {
		return tempest.Entitlement{}, errors.New("failed to parse received data from discord")
	}

	return res, nil
}

// For One-Time Purchase consumable SKUs, marks a given entitlement for the user as consumed.
// The entitlement will have consumed: true when using BaseClient.FetchEntitlements.
//
// https://discord.com/developers/docs/resources/entitlement#consume-an-entitlement
func (client *Client) ConsumeEntitlement(entitlementID tempest.Snowflake) error {
	_, err := client.Rest.Request(http.MethodPost, "/applications/"+client.ApplicationID.String()+"/entitlements/"+entitlementID.String()+"/consume", nil)
	return err
}

// https://discord.com/developers/docs/resources/entitlement#create-test-entitlement
func (client *Client) CreateTestEntitlement(payload tempest.TestEntitlementPayload) error {
	_, err := client.Rest.Request(http.MethodPost, "/applications/"+client.ApplicationID.String()+"/entitlements", payload)
	return err
}

// https://discord.com/developers/docs/resources/entitlement#delete-test-entitlement
func (client *Client) DeleteTestEntitlement(entitlementID tempest.Snowflake) error {
	_, err := client.Rest.Request(http.MethodDelete, "/applications/"+client.ApplicationID.String()+"/entitlements/"+entitlementID.String(), nil)
	return err
}

func (client *Client) tracef(format string, v ...any) {
	client.traceLogger.Printf("[CLIENT] "+format, v...)
}
