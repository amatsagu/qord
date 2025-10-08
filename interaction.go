package qord

import (
	"github.com/amatsagu/tempest"
)

// Used only for partial JSON parsing.
type InteractionTypeExtractor struct {
	Type tempest.InteractionType `json:"type"`
}

// Represents general interaction.
// Use Command/Component/Modal interaction to read Data field.
//
// https://discord.com/developers/docs/interactions/receiving-and-responding#interaction-object-interaction-structure
type Interaction struct {
	tempest.Interaction
	ShardID uint16  `json:"-"`
	Client  *Client `json:"-"`
}

// https://discord.com/developers/docs/interactions/receiving-and-responding#interaction-object
type CommandInteraction struct {
	*Interaction
	Data tempest.CommandInteractionData `json:"data"`
}

// https://discord.com/developers/docs/interactions/receiving-and-responding#interaction-object
type ComponentInteraction struct {
	*Interaction
	Data tempest.ComponentInteractionData `json:"data"`
}

// https://discord.com/developers/docs/interactions/receiving-and-responding#interaction-object
type ModalInteraction struct {
	*Interaction
	Data tempest.ModalInteractionData `json:"data"`
}
