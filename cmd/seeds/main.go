package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/jhchabran/tabloid"
	"github.com/jhchabran/tabloid/cmd"
	"github.com/jhchabran/tabloid/pgstore"
	"github.com/rs/zerolog/log"
)

var users = []string{"tintin", "milou", "haddock", "castafiore", "tournesol"}
var lorem = `Globular star cluster star stuff harvesting star light gathered by gravity take root and flourish vastness is bearable only through love Orion's sword. The only home we've ever known a still more glorious dawn awaits hearts of the stars culture a mote of dust suspended in a sunbeam a mote of dust suspended in a sunbeam. Courage of our questions two ghostly white figures in coveralls and helmets are softly dancing tingling of the spine courage of our questions made in the interiors of collapsing stars hearts of the stars.
Dispassionate extraterrestrial observer consciousness cosmic ocean preserve and cherish that pale blue dot brain is the seed of intelligence Hypatia? Circumnavigated the sky calls to us courage of our questions hearts of the stars take root and flourish how far away. Tendrils of gossamer clouds rich in heavy atoms vanquish the impossible another world with pretty stories for which there's little good evidence rich in heavy atoms? A very small stage in a vast cosmic arena courage of our questions descended from astronomers a very small stage in a vast cosmic arena tendrils of gossamer clouds Tunguska event.
Rogue white dwarf ship of the imagination of brilliant syntheses gathered by gravity from which we spring. Astonishment extraordinary claims require extraordinary evidence a mote of dust suspended in a sunbeam a mote of dust suspended in a sunbeam paroxysm of global death intelligent beings. Network of wormholes concept of the number one network of wormholes rich in heavy atoms the only home we've ever known realm of the galaxies.
Of brilliant syntheses culture the carbon in our apple pies something incredible is waiting to be known light years the only home we've ever known. Rings of Uranus paroxysm of global death laws of physics are creatures of the cosmos take root and flourish prime number. Extraplanetary Orion's sword permanence of the stars rich in heavy atoms invent the universe a still more glorious dawn awaits? Citizens of distant epochs Sea of Tranquility invent the universe with pretty stories for which there's little good evidence Sea of Tranquility Sea of Tranquility.
Globular star cluster Euclid tendrils of gossamer clouds another world venture bits of moving fluff. Made in the interiors of collapsing stars the only home we've ever known a still more glorious dawn awaits two ghostly white figures in coveralls and helmets are softly dancing hearts of the stars Sea of Tranquility? Permanence of the stars network of wormholes a still more glorious dawn awaits the ash of stellar alchemy the only home we've ever known invent the universe.
Quasar vastness is bearable only through love prime number dispassionate extraterrestrial observer Vangelis brain is the seed of intelligence. Muse about a very small stage in a vast cosmic arena the ash of stellar alchemy something incredible is waiting to be known something incredible is waiting to be known Sea of Tranquility? Concept of the number one Tunguska event hearts of the stars descended from astronomers extraordinary claims require extraordinary evidence hydrogen atoms.
Astonishment a billion trillion dispassionate extraterrestrial observer star stuff harvesting star light extraplanetary Orion's sword. Permanence of the stars vanquish the impossible prime number muse about take root and flourish permanence of the stars. Network of wormholes tingling of the spine a still more glorious dawn awaits something incredible is waiting to be known a mote of dust suspended in a sunbeam the carbon in our apple pies.
Flatland cosmic fugue hundreds of thousands prime number rich in heavy atoms kindling the energy hidden in matter? Great turbulent clouds inconspicuous motes of rock and gas something incredible is waiting to be known corpus callosum made in the interiors of collapsing stars two ghostly white figures in coveralls and helmets are softly dancing. Invent the universe take root and flourish at the edge of forever a very small stage in a vast cosmic arena citizens of distant epochs are creatures of the cosmos?
Encyclopaedia galactica Drake Equation laws of physics billions upon billions are creatures of the cosmos descended from astronomers? Great turbulent clouds a mote of dust suspended in a sunbeam Sea of Tranquility something incredible is waiting to be known finite but unbounded another world? The carbon in our apple pies permanence of the stars the sky calls to us the sky calls to us extraordinary claims require extraordinary evidence across the centuries.
Decipherment rogue cosmic fugue white dwarf worldlets not a sunrise but a galaxyrise? Bits of moving fluff take root and flourish cosmic ocean a very small stage in a vast cosmic arena dispassionate extraterrestrial observer are creatures of the cosmos. Something incredible is waiting to be known star stuff harvesting star light how far away star stuff harvesting star light Sea of Tranquility the only home we've ever known and billions upon billions upon billions upon billions upon billions upon billions upon billions.
We are the legacy of 15 billion years of cosmic evolution. We have a choice. We can enhance life and come to know the universe that made us, or we can squander our 15 billion year heritage in meaningless self-destruction. What happens in the first second of the next cosmic year depends on what we do, here and now, with our intelligence, and our knowledge of the cosmos.
`

func breakLorem() []string {
	strs := regexp.MustCompile("[!?.] ").Split(lorem, -1)
	var res []string
	for _, s := range strs {
		r := strings.TrimSpace(s)
		if len(r) > 50 {
			idx := 0
			for i, r := range s[50:] {
				if r == ' ' {
					idx = i
					break
				}
			}

			r = s[0 : 50+idx]
		}
		res = append(res, r)
	}

	return res
}

func main() {
	cfg := cmd.DefaultConfig()
	err := cfg.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot read configuration")
	}
	logger := cmd.SetupLogger(cfg)
	logger.Info().Msg("Seeding database")

	// setup database
	pgcfg := fmt.Sprintf(
		"user=%v dbname=%v sslmode=disable password=%v host=%v",
		cfg.DatabaseUser,
		cfg.DatabaseName,
		cfg.DatabasePassword,
		cfg.DatabaseHost,
	)
	pg := pgstore.New(pgcfg)
	err = pg.Connect()
	if err != nil {
		log.Fatal().Err(err).Msg("Can't connect to database")
	}

	// We're going to break the lorem string into multiple pieces, turn them into stories whose authors
	// are newly created users.
	strs := breakLorem()

	var userIDs []int64
	for _, u := range users {
		_, err := pg.CreateOrUpdateUser(u, u+"@gmail.com")
		if err != nil {
			log.Fatal().Err(err).Msg("Can't create user")
		}
	}

	// list all user ids so we can cycle through them to create stories
	rows, err := pg.DB().Query("SELECT id FROM users")
	if err != nil {
		log.Fatal().Err(err).Msg("Can't list users")
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		err = rows.Scan(&id)
		if err != nil {
			log.Fatal().Err(err).Msg("Can't list users")
		}
		userIDs = append(userIDs, id)
	}

	// let's now add the stories
	var stories []*tabloid.Story
	for i, title := range strs {
		authorID := userIDs[i%len(userIDs)]
		story := tabloid.NewStory(title, "", authorID, "https://duckduckgo.com")
		err = pg.InsertStory(story)
		if err != nil {
			log.Fatal().Err(err).Msg("Can't creates story")
		}

		stories = append(stories, story)
	}

	// let's add some comments on the stories
	for i, story := range stories {
		authorID := userIDs[i%len(userIDs)]
		body := strs[i%len(strs)]

		comment := tabloid.NewComment(story.ID, sql.NullInt64{}, body, authorID)
		err := pg.InsertComment(comment)
		if err != nil {
			log.Fatal().Err(err).Msg("Can't create comment")
		}

		// add some subcomments
		for j := 0; j < i%4; j++ {
			authorID := userIDs[j%len(userIDs)]
			body := strs[j%len(strs)]
			subcomment := tabloid.NewComment(story.ID, sql.NullInt64{Int64: comment.ID, Valid: true}, body, authorID)
			err := pg.InsertComment(subcomment)
			if err != nil {
				log.Fatal().Err(err).Msg("Can't create sub-comment")
			}

			for k := 0; k < i%3; k++ {
				authorID := userIDs[k%len(userIDs)]
				body := strs[k%len(strs)]
				subcomment := tabloid.NewComment(story.ID, sql.NullInt64{Int64: subcomment.ID, Valid: true}, body, authorID)
				err := pg.InsertComment(subcomment)
				if err != nil {
					log.Fatal().Err(err).Msg("Can't create sub-sub-comment")
				}
			}
		}
	}
}
