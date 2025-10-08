package qord

import (
	"encoding/json"

	"github.com/amatsagu/tempest"
)

func (client *Client) eventHandler(shardID uint16, packet EventPacket) {
	if client.customEventHandler != nil {
		client.customEventHandler(shardID, packet)
	}

	if packet.Event != INTERACTION_CREATE_EVENT {
		return
	}

	var interaction Interaction
	if err := json.Unmarshal(packet.Data, &interaction); err != nil {
		client.tracef("Received interaction event but failed to parse it: %v", err)
		return
	}

	interaction.ShardID = shardID
	interaction.Client = client

	switch interaction.Type {
	case tempest.APPLICATION_COMMAND_INTERACTION_TYPE,
		tempest.APPLICATION_COMMAND_AUTO_COMPLETE_INTERACTION_TYPE:
		var data tempest.CommandInteractionData
		if err := json.Unmarshal(interaction.Data, &data); err != nil {
			client.tracef("Received command/auto complete interaction event but failed to parse its data: %v", err)
			return
		}

		client.commandInteractionHandler(CommandInteraction{
			Interaction: &interaction,
			Data:        data,
		})
		return
	case tempest.MESSAGE_COMPONENT_INTERACTION_TYPE:
		var data tempest.ComponentInteractionData
		if err := json.Unmarshal(interaction.Data, &data); err != nil {
			client.tracef("Received component interaction event but failed to parse its data: %v", err)
			return
		}

		client.componentInteractionHandler(ComponentInteraction{
			Interaction: &interaction,
			Data:        data,
		})
		return
	case tempest.MODAL_SUBMIT_INTERACTION_TYPE:
		var data tempest.ModalInteractionData
		if err := json.Unmarshal(interaction.Data, &data); err != nil {
			client.tracef("Received modal interaction event but failed to parse its data: %v", err)
			return
		}

		client.modalInteractionHandler(ModalInteraction{
			Interaction: &interaction,
			Data:        data,
		})
		return
	}
}

func undefinedMessageHandler(msg Message) {
	msg.Client.tracef("You see this trace message because client received text message but there's no defined handler for it.")
}

func undefinedCommandHandler(itx CommandInteraction) {
	itx.Client.tracef("You see this trace message because client received slash command, user command, message command or command auto complete interaction but there's no defined handler for it.")
}

func undefinedComponentHandler(itx ComponentInteraction) {
	itx.Client.tracef("You see this trace message because client received component interaction but there's no defined handler for it.")
}

func undefinedModalHandler(itx ModalInteraction) {
	itx.Client.tracef("You see this trace message because client received modal interaction but there's no defined handler for it.")
}
