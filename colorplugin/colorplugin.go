package colorplugin

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/todd-beckman/mmmorty"
)

const colorCommand = "color me"
const manageColorCommand = "manage color"
const stopManagingCommand = "stop managing"

type ColorPlugin struct {
	bot          *mmmorty.Bot
	managedRoles map[string]bool `json: "managedRoles"`
}

// Used to determine if the role is more than aesthetic
var authPermissions = 0x00000002 | // kick
	0x00000004 | // ban
	0x00000008 | // admin
	0x00000010 | // manage channels
	0x00000020 | // manage guild
	0x00000080 | // view audit log
	0x00002000 | // manage messages
	0x00400000 | // mute members in voice channel
	0x00800000 | // deafen members in voice channel
	0x01000000 | // move members between channels
	0x08000000 | // modify others' nicknames
	0x10000000 | // manage roles
	0x20000000 | // manage webhooks
	0x40000000 //   manage emojis

func doesRoleHaveAuth(permissions int) bool {
	return permissions&authPermissions > 0
}

func (p *ColorPlugin) printableRoles() []string {
	printableRoles := []string{}
	for role, isManaged := range p.managedRoles {
		if isManaged {
			printableRoles = append(printableRoles, role)
		}
	}
	return printableRoles
}

func (p *ColorPlugin) Help(bot *mmmorty.Bot, service mmmorty.Service, message mmmorty.Message, detailed bool) []string {
	help := mmmorty.CommandHelp(service, colorCommand, "<color>", "assigns the desired color if it is avialable")
	help = append(help, mmmorty.CommandHelp(service, manageColorCommand, "<color list>", "remembers each of these roles so they can be removed when a user changes color")[0])
	help = append(help, mmmorty.CommandHelp(service, stopManagingCommand, "<color>", "stops managing the given color")[0])
	return help
}

func (p *ColorPlugin) Load(bot *mmmorty.Bot, service mmmorty.Service, data []byte) error {
	if data != nil {
		if err := json.Unmarshal(data, p); err != nil {
			log.Println("Error loading data", err)
		}
	}

	return nil
}

func (p *ColorPlugin) Message(bot *mmmorty.Bot, service mmmorty.Service, message mmmorty.Message) {
	defer mmmorty.MessageRecover()

	if service.Name() != mmmorty.DiscordServiceName {
		return
	}

	if service.IsMe(message) {
		return
	}

	if mmmorty.MatchesCommand(service, "color me", message) {
		p.handleColorMe(bot, service, message)
	} else if mmmorty.MatchesCommand(service, "manage color", message) {
		p.handleManageColor(bot, service, message)
	} else if mmmorty.MatchesCommand(service, "stop managing", message) {
		p.handleStopManaging(bot, service, message)
	}
}

func (p *ColorPlugin) handleColorMe(bot *mmmorty.Bot, service mmmorty.Service, message mmmorty.Message) {
	_, parts := mmmorty.ParseCommand(service, message)

	requester := fmt.Sprintf("<@%s>", message.UserID())

	if len(parts) == 1 {
		reply := fmt.Sprintf("Uh, %s, I think you forgot to name a color.", requester)
		service.SendMessage(message.Channel(), reply)
		return
	} else if len(parts) > 2 {
		reply := fmt.Sprintf("Uh, %s, I can't give you more than one color.", requester)
		service.SendMessage(message.Channel(), reply)
		return
	}

	color := strings.ToLower(parts[1])

	discord := service.(*mmmorty.Discord)
	role := discord.GetRoleByName(message.Channel(), color)

	if role == nil {
		reply := fmt.Sprintf("Uh, %s, I can't find a role called %s", requester, color)
		service.SendMessage(message.Channel(), reply)
		return
	}

	if doesRoleHaveAuth(role.Permissions) {
		reply := fmt.Sprintf("Uh, %s, I think %s is more than just a colored role.", requester, color)
		service.SendMessage(message.Channel(), reply)
		return
	}

	userRoles := discord.UserRoles(message.Channel(), message.UserID())
	for _, userRole := range userRoles {
		for r, isManaged := range p.managedRoles {
			if !isManaged {
				continue
			}

			managedRole := discord.GetRoleByName(message.Channel(), r)
			if userRole == managedRole.ID {
				ok := discord.GuildMemberRoleRemove(message.Channel(), message.UserID(), userRole)
				if !ok {
					reply := fmt.Sprintf("Uh, %s, something went wrong. Are you sure I can manage %v?", requester, color)
					service.SendMessage(message.Channel(), reply)
					continue
				}
			}
		}
	}

	ok := discord.GuildMemberRoleAdd(message.Channel(), message.UserID(), role.ID)
	if !ok {
		reply := fmt.Sprintf("Uh, %s, something went wrong. Are you sure I can let you be %v?", requester, color)
		service.SendMessage(message.Channel(), reply)
		return
	}

	reply := fmt.Sprintf("You got it, %s! You are now %s", requester, color)
	service.SendMessage(message.Channel(), reply)
	return

}

func (p *ColorPlugin) handleManageColor(bot *mmmorty.Bot, service mmmorty.Service, message mmmorty.Message) {
	discord := service.(*mmmorty.Discord)

	requester := fmt.Sprintf("<@%s>", message.UserID())

	if message.UserID() != discord.OwnerUserID {
		reply := fmt.Sprintf("Uh, %s, I think you need to ask my Rick for that command.", requester)
		service.SendMessage(message.Channel(), reply)
		return
	}

	_, parts := mmmorty.ParseCommand(service, message)

	if len(parts) == 1 {
		reply := fmt.Sprintf("Uh, %s, I think you forgot to name a color.", requester)
		service.SendMessage(message.Channel(), reply)
		return
	}

	for _, c := range parts[1:] {
		color := strings.ToLower(c)
		if p.managedRoles[color] {
			reply := fmt.Sprintf("Uh, %s, I am already managing %s", requester, color)
			service.SendMessage(message.Channel(), reply)
			continue
		}

		role := discord.GetRoleByName(message.Channel(), color)

		if role == nil {
			reply := fmt.Sprintf("Uh, %s, I can't find a role called %s", requester, color)
			service.SendMessage(message.Channel(), reply)
			continue
		}

		if doesRoleHaveAuth(role.Permissions) {
			reply := fmt.Sprintf("Uh, %s, I think %s is more than just a colored role.", requester, color)
			service.SendMessage(message.Channel(), reply)
			continue
		}

		p.managedRoles[color] = true
	}

	printableRoles := p.printableRoles()
	reply := fmt.Sprintf("Uh, I guess that means I am managing %v now.", printableRoles)
	service.SendMessage(message.Channel(), reply)
}

func (p *ColorPlugin) handleStopManaging(bot *mmmorty.Bot, service mmmorty.Service, message mmmorty.Message) {
	discord := service.(*mmmorty.Discord)

	requester := fmt.Sprintf("<@%s>", message.UserID())

	if message.UserID() != discord.OwnerUserID {
		reply := fmt.Sprintf("Uh, %s, I think you need to ask my Rick for that command.", requester)
		service.SendMessage(message.Channel(), reply)
		return
	}

	_, parts := mmmorty.ParseCommand(service, message)

	if len(parts) == 1 {
		reply := fmt.Sprintf("Uh, %s, I think you forgot to name a color.", requester)
		service.SendMessage(message.Channel(), reply)
		return
	}

	for _, c := range parts[1:] {
		color := strings.ToLower(c)
		if !p.managedRoles[color] {
			reply := fmt.Sprintf("Uh, %s, I'm not managing %s", requester, color)
			service.SendMessage(message.Channel(), reply)
			continue
		}

		delete(p.managedRoles, color)
	}

	printableRoles := p.printableRoles()
	reply := fmt.Sprintf("Uh, I guess that means I am managing %v now.", printableRoles)
	service.SendMessage(message.Channel(), reply)
}

// Save will save plugin state to a byte array.
func (p *ColorPlugin) Save() ([]byte, error) {
	return json.Marshal(p)
}

// Stats will return the stats for a plugin.
func (p *ColorPlugin) Stats(bot *mmmorty.Bot, service mmmorty.Service, message mmmorty.Message) []string {
	return []string{}
}

// Name returns the name of the plugin.
func (p *ColorPlugin) Name() string {
	return "Color"
}

// New will create a new Reminder plugin.
func New() mmmorty.Plugin {
	p := &ColorPlugin{
		managedRoles: map[string]bool{},
	}
	return p
}
