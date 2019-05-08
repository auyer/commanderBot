package bot

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/auyer/massmoverbot/mover"
	"github.com/auyer/massmoverbot/utils"
	"github.com/bwmarrin/discordgo"
)

// Summon command moves all users to specified channel
func (bot *Bot) Summon(m *discordgo.MessageCreate, params []string) (string, error) {
	workersChannel := make(chan []*discordgo.Session, 1)
	go utils.DetectServants(m.GuildID, append(bot.PowerupSessions, bot.CommanderSession), workersChannel)

	guild, _ := bot.CommanderSession.Guild(m.GuildID)

	// Get the Authors current voice channel
	var destination string
	for _, member := range guild.VoiceStates {
		if member.UserID == m.Author.ID {
			destination = member.ChannelID
			break
		}
	}

	guildLocale := bot.Messages[utils.GetGuildLocale(bot.DB, m)]
	// Check if the Authors channel exists
	if destination == "" {
		_, _ = bot.CommanderSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf(guildLocale["CantFindUser"], m.Author.Username))
		return "", errors.New("user not connected to any voice channel")
	}

	// Check Authors permission for voice channel
	if !utils.CheckPermissions(bot.CommanderSession, destination, m.Author.ID, discordgo.PermissionVoiceMoveMembers) {
		_, _ = bot.CommanderSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf(guildLocale["NoPermissionsDestination"]))
		return "", errors.New("no permission destination")
	}

	numParams := len(params)
	if numParams == 2 {
		origin, err := getOrigin(guild.Channels, params[1])
		if err != nil {
			_, _ = bot.CommanderSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf(guildLocale["CantFindChannel"], params[1]))
			return "", err
		}

		return moveOriginDestination(workersChannel, guild, origin, destination, false)
	} else if numParams == 3 && params[2] == "1" {
		origin, err := getOrigin(guild.Channels, params[1])
		if err != nil {
			_, _ = bot.CommanderSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf(guildLocale["CantFindChannel"], params[1]))
			return "", err
		}

		return moveOriginDestination(workersChannel, guild, origin, destination, true)
	} else {
		_, _ = bot.CommanderSession.ChannelMessageSend(m.ChannelID, fmt.Sprintf(guildLocale["SummonHelp"], bot.Prefix, bot.Prefix, bot.Prefix, bot.Prefix, bot.Prefix, utils.ListChannelsForHelpMessage(guild.Channels)))
	}

	return "", nil
}

// MoveOriginDestination function moves discord users
func moveOriginDestination(workersChannel chan []*discordgo.Session, guild *discordgo.Guild, origin string, destination string, moveAfk bool) (string, error) {
	if origin == destination {
		return "", errors.New("destination and origin are the same")
	}

	num := 0
	var wg sync.WaitGroup
	sessions := <-workersChannel
	for index, member := range guild.VoiceStates {
		if origin == "" || member.ChannelID == origin {
			if !moveAfk && member.ChannelID == guild.AfkChannelID {
				continue
			}

			wg.Add(1)
			go func(guildID, userID, dest string, servants []*discordgo.Session, index int) {
				defer wg.Done()
				err := mover.MoveAndRetry(servants[index%len(servants)], guildID, userID, dest, 3)
				if err != nil {
					log.Println("Failed to move user with ID: "+userID, err)
				}
			}(guild.ID, member.UserID, destination, sessions, index)

			num++
		}
	}

	wg.Wait()
	return strconv.Itoa(num), nil
}

func getOrigin(channels []*discordgo.Channel, param string) (string, error) {
	if param == "0" {
		return "", nil
	}

	return utils.GetChannel(channels, param)
}