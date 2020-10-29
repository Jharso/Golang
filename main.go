package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// cache tables
var guildIDs map[string]string
var usernames map[string]discordgo.User

func main() {
	dg, err := discordgo.New("Bot " + os.Getenv("SHUFFLEBOT_TOKEN"))
	if err != nil {
		fmt.Println("Error creando el bot de Discord: ", err)
		return
	}

	guildIDs = make(map[string]string)
	usernames = make(map[string]discordgo.User)
	dg.AddHandler(messageHandler)
	dg.AddHandler(userPresenceUpdateHandler)

	dg.Open()
	if err != nil {
		fmt.Println("Error al abrir conexión: ", err)
		return
	}

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func sendReply(s *discordgo.Session, m *discordgo.MessageCreate, str string) {
	sendMessage := fmt.Sprintf("<@!%s> ", m.Author.ID)
	sendMessage += str
	s.ChannelMessageSend(m.ChannelID, sendMessage)
}

func isContain(needle string, haystack []string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func userPresenceUpdateHandler(s *discordgo.Session, p *discordgo.PresenceUpdate) {
	if p.User.Username != "" {
		// actualiza cache
		fmt.Println("Nombre de usuario modificado: " + p.User.Username)
		usernames[p.User.ID] = *p.User
	}
}

func messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	// translate channelID -> guildID to reduce latency
	// This does not need use in case of building with latest discordgo's develop branch
	gid, ok := guildIDs[m.ChannelID]
	if !ok {
		fmt.Println("Cache PERDIDO")
		// cache miss
		sourceTextChannel, err := s.Channel(m.ChannelID)
		if err != nil {
			fmt.Println("Error al obtener el canal de origen: ", err)
			return
		}
		gid = sourceTextChannel.GuildID
		guildIDs[m.ChannelID] = gid
	}

	if gid == "" {
		// Invoked from user chat directly
		s.ChannelMessageSend(m.ChannelID, "Envíe después de conectarse y unirse a algún canal de voz.")
		return
	}

	// Invoked from Server (Guild)

	if !strings.HasPrefix(m.Content, "!!teams") {
		return
	}

	args := strings.Split(m.Content, " ")
	if len(args) <= 1 {
		sendReply(s, m, "Uso: `!!teams <número de equipos a crear> [omitir nombre de usuario...]`")
		return
	}

	var skipUsernames []string
	if len(args) > 2 {
		skipUsernames = args[2:len(args)]
	}

	_nTeams, err := strconv.ParseInt(args[1], 10, 32)
	if err != nil {
		fmt.Println("Error al analizar el valor especificado por el usuario: ", err)
		sendReply(s, m, "¡Por favor especifique en número!")
		return
	}

	nTeams := int(_nTeams)

	if nTeams <= 0 || nTeams >= 100 {
		if gid == "223518751650217994" {
			// for internal uses
			sendReply(s, m, "<:kakattekoi:461046115257679872>")
		} else {
			sendReply(s, m, "Por favor, especifique en número * realista * !!!!!")
		}
		return
	}

	guild, err := s.Guild(gid)
	if err != nil {
		fmt.Println("Error al obtener el gremio: ", err)
		return
	}

	// find users voice channel & fetch connected users
	voiceChannelUsers := map[string][]string{}
	var sourceVoiceChannel string
	for _, vs := range guild.VoiceStates {
		if vs.UserID == m.Author.ID {
			sourceVoiceChannel = vs.ChannelID
		}

		// check cache
		user, ok := usernames[vs.UserID]
		if !ok {
			// cache MISS
			u, err := s.User(vs.UserID)
			if err != nil {
				fmt.Println("Error al obtener el nombre de usuario")
				sendReply(s, m, "Error: error desconocido.")
				return
			}
			user = *u
			usernames[vs.UserID] = user
		}

		if !isContain(user.Username, skipUsernames) {
			voiceChannelUsers[vs.ChannelID] =
				append(voiceChannelUsers[vs.ChannelID], user.Username)
		}
	}

	// not found in any voice channel
	if sourceVoiceChannel == "" {
		sendReply(s, m, "Conectarse a algún canal de voz!")
		fmt.Println(m.Content)
		return
	}

	// check nTeams
	totalUserCount := len(voiceChannelUsers[sourceVoiceChannel])

	nMembers := int(math.Round(float64(totalUserCount) / float64(nTeams)))
	if totalUserCount < nTeams {
		sendReply(s, m, fmt.Sprintf("¡Se requieren más miembros para hacer %d equipo(s) por %d miembro(s)", nTeams, nMembers))
		return
	}

	// shuffle by connected users
	idx := rand.Perm(totalUserCount)

	var shuffledUsers []string
	for _, newIdx := range idx {
		shuffledUsers = append(shuffledUsers, voiceChannelUsers[sourceVoiceChannel][newIdx])
	}

	// devide into {nTeams} teams
	result := make([][]string, nTeams)
	for i := 0; i < nTeams-1; i++ {
		result[i] = shuffledUsers[i*nMembers : (i+1)*nMembers]
	}
	result[nTeams-1] = shuffledUsers[(nTeams-1)*nMembers : len(shuffledUsers)]
	fmt.Println(result)

	// send message
	outputString := fmt.Sprintf("Se crearon %d equipos!\n", nTeams)
	for i := 0; i < nTeams; i++ {
		outputString += fmt.Sprintf("Equipo %d: %s\n", i+1, strings.Join(result[i], ", "))
	}
	sendReply(s, m, outputString)
}
