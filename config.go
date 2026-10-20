package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Config is everything editable from /config. Text, list, number and flag
// fields live in maps keyed by the registries below, so adding a new knob is
// a one-line registry entry and the admin UI picks it up automatically.
type Config struct {
	SetupDone   bool                `json:"setupDone"`
	Nickname    string              `json:"nickname"`
	SignOff     string              `json:"signOff"`
	BirthBSYear int                 `json:"birthBSYear"`
	BSMonth     int                 `json:"bsMonth"`
	BSDay       int                 `json:"bsDay"`
	Timezone    string              `json:"timezone"`
	TeaseDays   int                 `json:"teaseDays"`
	Overrides   map[int]string      `json:"overrides"`  // BS year → "2006-01-02"
	ForceState  string              `json:"forceState"` // "", pre, during, post — for everyone
	Flags       map[string]bool     `json:"flags"`
	Nums        map[string]float64  `json:"nums"`
	Texts       map[string]string   `json:"texts"`
	Lists       map[string][]string `json:"lists"`
	Milestones  []Milestone         `json:"milestones"`
	Photos      []Photo             `json:"photos"`
}

type Milestone struct {
	Count int    `json:"count"`
	Text  string `json:"text"`
}

type Photo struct {
	ID      string `json:"id"` // file name inside DATA_DIR/photos
	Caption string `json:"caption"`
}

// Stage controls spoilers: "during" content is only sent to the public site
// once the birthday has started, "post" content once it is over.
const (
	stageAlways = ""
	stageDuring = "during"
	stagePost   = "post"
)

type TextDef struct {
	Key, Section, Label, Help string
	Multiline, Required       bool
	Stage                     string
	Default                   string
}

type ListDef struct {
	Key, Section, Label, Help string
	Stage                     string
	Min                       int
	Default                   []string
}

type NumDef struct {
	Key, Section, Label, Help, Unit string
	Min, Max, Step, Default         float64
}

type FlagDef struct {
	Key, Section, Label, Help string
	Default                   bool
}

var textDefs = []TextDef{
	// Countdown
	{Key: "pre.title", Section: "Countdown (pre)", Label: "Title", Default: "BIRTHDAY AFTER"},
	{Key: "pre.sub", Section: "Countdown (pre)", Label: "Subtext", Multiline: true, Default: "Intazar do kya appu ajhai alik time 6a"},
	{Key: "pre.scroll", Section: "Countdown (pre)", Label: "Scroll hint", Default: "Tala hera, khelauna kura chha 👇"},

	// Birthday
	{Key: "during.title", Section: "Birthday (during)", Label: "Title", Help: "{NAME} is her nickname in capitals.", Stage: stageDuring, Default: "HAPPY BIRTHDAY {NAME}"},
	{Key: "during.sub", Section: "Birthday (during)", Label: "Subtext", Multiline: true, Stage: stageDuring, Default: "Janmadin ko dherai dherai subhakamana mero mutu. Aune din haru timro ajhai mitho ra maya le bhareko hoss."},
	{Key: "during.age", Section: "Birthday (during)", Label: "Age line", Help: "{age} is worked out from her birth year. Leave empty to hide.", Stage: stageDuring, Default: "Aaja timi {age} barsa pugyau 🥹"},
	{Key: "cake.title", Section: "Birthday (during)", Label: "Cake title", Stage: stageDuring, Default: "Candle phuu gara! 🎂"},
	{Key: "cake.hint", Section: "Birthday (during)", Label: "Cake hint", Stage: stageDuring, Default: "Mic ma phuu gara, natra candle ma tap gara"},
	{Key: "cake.mic", Section: "Birthday (during)", Label: "Mic button", Stage: stageDuring, Default: "🎤 Mic on gara"},
	{Key: "cake.again", Section: "Birthday (during)", Label: "Relight button", Stage: stageDuring, Default: "Feri phuu garne? 🕯️"},
	{Key: "wish.title", Section: "Birthday (during)", Label: "Wish title", Stage: stageDuring, Default: "Euta wish maag ✨"},
	{Key: "wish.placeholder", Section: "Birthday (during)", Label: "Wish placeholder", Stage: stageDuring, Default: "Mero wish ho ki..."},
	{Key: "wish.button", Section: "Birthday (during)", Label: "Wish button", Stage: stageDuring, Default: "Udaau 🏮"},
	{Key: "wish.done", Section: "Birthday (during)", Label: "After the wish", Stage: stageDuring, Default: "Wish aakash tira gayo ✨"},
	{Key: "gift.title", Section: "Birthday (during)", Label: "Gift title", Stage: stageDuring, Default: "Timro lagi euta gift 🎁"},
	{Key: "gift.hint", Section: "Birthday (during)", Label: "Gift hint", Stage: stageDuring, Default: "Tap tap garera khola"},
	{Key: "gift.message", Section: "Birthday (during)", Label: "Gift message", Help: "What's inside the gift box: a coupon, a promise, a hint about the real present.", Multiline: true, Required: true, Stage: stageDuring},
	{Key: "letter.title", Section: "Birthday (during)", Label: "Letter title", Stage: stageDuring, Default: "Timilai euta chitthi 💌"},
	{Key: "letter.body", Section: "Birthday (during)", Label: "Birthday letter", Help: "Types itself out on her birthday, signed with your name.", Multiline: true, Required: true, Stage: stageDuring},
	{Key: "slideshow.title", Section: "Birthday (during)", Label: "Slideshow title", Stage: stageDuring, Default: "Hamro yaadharu 📸"},

	// After
	{Key: "post.title", Section: "After (post)", Label: "Title", Stage: stagePost, Default: "SAKYO BIRTHDAY"},
	{Key: "post.sub", Section: "After (post)", Label: "Subtext", Multiline: true, Stage: stagePost, Default: "LAAAAA! ajhai ek barsa badhyo ! Budho huna atis hai, ma jastai tanneri basna mehenat gar ajhai la !"},
	{Key: "post.ago", Section: "After (post)", Label: "Under the timer", Stage: stagePost, Default: "agadi sakiyo 😌"},
	{Key: "hangover.hint", Section: "After (post)", Label: "Hangover hint", Stage: stagePost, Default: "Party sakiyo... confetti safa gara na 🧹 (swipe gara)"},
	{Key: "hangover.done", Section: "After (post)", Label: "All swept", Stage: stagePost, Default: "Wah, safa bhayo! Thank you 🥰"},
	{Key: "budhi.title", Section: "After (post)", Label: "Budhi meter title", Stage: stagePost, Default: "Budhi meter 👵"},
	{Key: "budhi.button", Section: "After (post)", Label: "Budhi button", Stage: stagePost, Default: "Aru budhi banau 👵"},
	{Key: "budhi.done", Section: "After (post)", Label: "At 100%", Stage: stagePost, Default: "100% budhi! Tara mero lagi sadhai cute 💗"},
	{Key: "leftover.title", Section: "After (post)", Label: "Leftover cake title", Stage: stagePost, Default: "Bacheko cake 🍰"},
	{Key: "leftover.hint", Section: "After (post)", Label: "Leftover cake hint", Stage: stagePost, Default: "Euta euta slice khau"},
	{Key: "rate.title", Section: "After (post)", Label: "Rating title", Stage: stagePost, Default: "Birthday lai rating deu ⭐"},
	{Key: "rate.placeholder", Section: "After (post)", Label: "Review placeholder", Stage: stagePost, Default: "Boyfriend ko performance kasto thiyo? 😌"},
	{Key: "rate.button", Section: "After (post)", Label: "Rating button", Stage: stagePost, Default: "Pathau 💌"},
	{Key: "rate.thanks", Section: "After (post)", Label: "After rating", Stage: stagePost, Default: "Review pathayo! 5 star nai ho ni? 😌"},

	// Envelope
	{Key: "envelope.to", Section: "Envelope", Label: "Envelope label", Default: "{name} ko lagi 💌"},
	{Key: "envelope.hint", Section: "Envelope", Label: "Tap hint", Default: "Kholna tap gara"},
	{Key: "envelope.button", Section: "Envelope", Label: "Close button", Default: "Bhitra jau 💗"},
	{Key: "envelope.birthday", Section: "Envelope", Label: "Birthday note", Help: "Shown in the envelope on her birthday instead of the daily notes.", Multiline: true, Stage: stageDuring, Default: "Aaja timro din! Happy birthday {name} 💗 Bhitra jau, surprise haru parkhirachhan."},

	// Toys
	{Key: "toys.title", Section: "Toys", Label: "Toys heading (birthday & after)", Default: "Khelauna kura 🧸"},
	{Key: "hug.title", Section: "Toys", Label: "Hug meter title", Default: "Hug meter 🤗"},
	{Key: "hug.button", Section: "Toys", Label: "Hug button", Default: "Hug!"},
	{Key: "question.text", Section: "Toys", Label: "The question", Default: "Sadhai mero bhayera basne? 🥺💗"},
	{Key: "question.yes", Section: "Toys", Label: "Yes button", Default: "Ho! 💗"},
	{Key: "question.no", Section: "Toys", Label: "No button", Default: "Hoina"},
	{Key: "question.after", Section: "Toys", Label: "After she says yes", Default: "Thaha thiyo nii! Aba ta forever booked 💍"},
	{Key: "jar.title", Section: "Toys", Label: "Jar title", Default: "Maya ko jar 🫙"},
	{Key: "jar.hint", Section: "Toys", Label: "Jar hint", Default: "Tap gara, euta karan niklanchha"},
	{Key: "jar.empty", Section: "Toys", Label: "When all are read", Default: "Sabai padhyau! Feri suru garchhu... 💗"},
	{Key: "photos.title", Section: "Toys", Label: "Photo wall title", Default: "Hamro yaadharu 📸"},

	{Key: "footer", Section: "Footer", Label: "Footer", Default: "{from} le maya garera banako 💗"},
}

var listDefs = []ListDef{
	{Key: "envelope.notes", Section: "Envelope", Label: "Daily envelope notes", Help: "A different one each day before her birthday.", Min: 3, Default: []string{
		"Good morning {name} ☀️ Aaja pani timilai dherai maya garchhu.",
		"Timi hasda sansar nai ramro dekhinchha 💗",
		"Aaja pani timro yaad aayo... sadhai jasto 🙈",
		"Pani piyau hai! Ani khana time ma khau 🍚",
		"Timi mero favourite notification hau 📱💗",
		"Birthday najik aaudai chha... ready chhau? 🎂",
		"Kehi garo bhaye pani ma chhu nii timro saath 🤝",
		"Timro gaala tanna man lagyo aaja 🥹",
		"Ek din ma kati choti maya laagchha thaha chha? Ganna sakdina 😌",
		"Aaja euta hug pending chha, collect garna aau 🤗",
		"Timi bina ta momo pani mitho laagdaina 🥟",
		"Ekchoti smile gara ta 😊 ... ho, tyahi!",
	}},
	{Key: "envelope.post", Section: "Envelope", Label: "Envelope notes after the birthday", Stage: stagePost, Default: []string{
		"Birthday sakyo tara maya sakeko chhaina 😌",
		"Ek barsa budhi bhayau, tara ajhai cute 👵💗",
		"Cake bachyo ki? Malai pani dinu 🍰",
		"Party hangover sakiyo ki ajhai? 😴",
	}},
	{Key: "reasons", Section: "Toys", Label: "Reasons jar", Help: "One reason pops out per tap.", Min: 5, Default: []string{
		"Timro hasai 😊",
		"Timi risaunda pani cute dekhinchhau",
		"Timi mero kura dhyan le sunchhau",
		"Timi mero khyal garchhau",
		"Timro hug sabai bhanda nyano 🤗",
		"Timi mero best friend pani hau",
		"Timi sanga kahile bore laagdaina",
		"Timro voice note haru 🎧",
		"Timi le malai ramro manchhe banauchhau",
		"Mitho khana dekhda timi kati khusi hunchhau 🍜",
		"Timro gaala 🥹",
		"Timi sanga ko late night kura haru 🌙",
		"Timi mero naramro jokes ma pani haschhau",
		"Timro aankha ✨",
		"Timi dherai strong chhau",
		"Timi kati mehenati chhau",
		"Timi sanga future dekhchhu 🏡",
		"Timi le mero mood thik parchhau",
		"Timi jasto aru koi chhaina",
		"Timi sanga ladai garda pani maya laagchha",
		"Kina bhane timi {name} hau 💗",
	}},
	{Key: "bear.lines", Section: "Bear", Label: "Bear says (when petted)", Default: []string{
		"Hehe, kutkuti laagyo 🙈",
		"{name}! 💗",
		"Malai pani hug chahiyo 🤗",
		"Birthday kati din baki? 😋",
		"Ma timro bubu hu 🐻",
		"Feri tap gara na",
		"Aaja ko maya: 100% 💯",
		"Timi cute chhau, thaha chha? 😌",
		"Muah! 😘",
		"Khana khayau? 🍚",
	}},
	{Key: "bear.grumpy", Section: "Bear", Label: "Bear gets grumpy (spam)", Default: []string{
		"Basa na! 😤",
		"Eh, kati thichchheu? 😠",
		"Dukhyo nii! 😾",
		"Ali shanti! 🙄",
	}},
	{Key: "bear.forgive", Section: "Bear", Label: "Bear forgives", Default: []string{
		"Thik chha, maaf garide 🥺",
		"Jaha bhayo, aba maya gara 💗",
		"Huss, risayeko chhaina 😌",
	}},
	{Key: "question.pleas", Section: "Toys", Label: "Runaway No button pleas", Help: "The No button cycles through these as it runs away.", Default: []string{
		"Pakka? 🥺",
		"Feri soch na...",
		"Galti le thichyau hola",
		"Hora? 😢",
		"Ma runchhu hai!",
		"Last chance!",
		"Yo button bigryo 🙈",
		"Yes thich na la",
	}},
	{Key: "budhi.jokes", Section: "After (post)", Label: "Budhi meter jokes", Help: "One per step. Each step adds an accessory and ages the bear a bit.", Stage: stagePost, Default: []string{
		"Chasma lagau, aba sano akshar padhna garo 👓",
		"Kapal fulna thalyo 🧓",
		"Aba lauro chahiyo 🦯",
		"Chiya bina din chaldaina 🍵",
		"Saal odhera ghaam taapne 🧣",
		"'Hamro jamana ma...' suru bhayo 📻",
	}},
	{Key: "leftover.slices", Section: "After (post)", Label: "Leftover cake slices", Help: "One slice per line; the last one is the sweet one.", Stage: stagePost, Default: []string{
		"Yum! 😋",
		"Arko pani? 🍰",
		"Diet bholi dekhi 😌",
		"Crumbs sabai tira pugyo 🙈",
		"Bas euta matra baki...",
		"Last slice: yo cake jastai mero maya pani sabai timro 💗",
	}},
}

var numDefs = []NumDef{
	{Key: "cake.candles", Section: "Cake & mic", Label: "Candles", Min: 1, Max: 12, Step: 1, Default: 5},
	{Key: "mic.sensitivity", Section: "Cake & mic", Label: "Mic sensitivity", Help: "Higher = a softer blow is enough.", Min: 1, Max: 10, Step: 1, Default: 6},
	{Key: "mic.blowMs", Section: "Cake & mic", Label: "Blow length", Help: "How long she has to keep blowing to put out one candle.", Unit: "ms", Min: 100, Max: 2000, Step: 50, Default: 300},
	{Key: "slideshow.seconds", Section: "Timing", Label: "Slideshow speed", Unit: "s per photo", Min: 2, Max: 15, Step: 0.5, Default: 4},
	{Key: "letter.speed", Section: "Timing", Label: "Letter typing speed", Unit: "ms per letter", Min: 5, Max: 200, Step: 5, Default: 40},
	{Key: "bear.autoSwap", Section: "Timing", Label: "Bear swaps GIF every", Help: "0 = only when petted.", Unit: "s", Min: 0, Max: 60, Step: 1, Default: 8},
}

// Features that the spam guard watches, with default (max taps, window s, lock s).
var spamFeatures = []struct {
	Key, Label        string
	Max, Window, Lock float64
}{
	{"bear", "Petting the bear", 8, 3, 3},
	{"hug", "Hug button", 30, 4, 4},
	{"jar", "Reasons jar", 5, 4, 4},
	{"yes", "Yes button", 6, 3, 3},
	{"photos", "Opening photos", 8, 4, 3},
	{"budhi", "Budhi meter", 8, 3, 3},
}

var flagDefs = []FlagDef{
	{Key: "envelope", Section: "Everywhere", Label: "Daily envelope", Default: true},
	{Key: "tapHearts", Section: "Everywhere", Label: "Hearts on every tap", Default: true},
	{Key: "floatingHearts", Section: "Everywhere", Label: "Floating background hearts", Default: true},
	{Key: "vibrate", Section: "Everywhere", Label: "Vibrate on hugs & milestones", Default: true},
	{Key: "spamGuard", Section: "Everywhere", Label: "Grumpy bear when she spams", Default: true},
	{Key: "bear", Section: "Toys", Label: "Pet the bear", Default: true},
	{Key: "hug", Section: "Toys", Label: "Hug meter", Default: true},
	{Key: "question", Section: "Toys", Label: "Runaway No question", Default: true},
	{Key: "jar", Section: "Toys", Label: "Reasons jar", Default: true},
	{Key: "photos", Section: "Toys", Label: "Polaroid wall", Default: true},
	{Key: "toysOnBirthday", Section: "Toys", Label: "Show toys on the birthday & after pages too", Default: true},
	{Key: "mic", Section: "Birthday", Label: "Blow out candles with the mic", Help: "Needs HTTPS (or localhost). Tapping always works.", Default: true},
	{Key: "music", Section: "Birthday", Label: "Music-box Happy Birthday", Default: true},
	{Key: "wish", Section: "Birthday", Label: "Make-a-wish lantern", Default: true},
	{Key: "gift", Section: "Birthday", Label: "Gift box", Default: true},
	{Key: "letter", Section: "Birthday", Label: "Birthday letter", Default: true},
	{Key: "slideshow", Section: "Birthday", Label: "Photo slideshow", Default: true},
	{Key: "hangover", Section: "After", Label: "Hangover scene", Default: true},
	{Key: "budhi", Section: "After", Label: "Budhi meter", Default: true},
	{Key: "leftover", Section: "After", Label: "Leftover cake", Default: true},
	{Key: "rating", Section: "After", Label: "Rate your birthday", Default: true},
}

var defaultMilestones = []Milestone{
	{10, "10 hugs! Warm-up sakiyo 🤗"},
	{25, "25! Ma ta pagliye 🥴"},
	{50, "50 hugs! Chest crush bhayo 😵"},
	{100, "100!! Hug champion 🏆"},
	{250, "250... saas pherna deu 😮‍💨"},
	{500, "500! Aba ta forever hug contract sign 📝💗"},
	{1000, "1000!!! Timi pagal, ma pani pagal 💗"},
}

func init() {
	for _, f := range spamFeatures {
		sec := "Spam guard"
		numDefs = append(numDefs,
			NumDef{Key: "spam." + f.Key + ".max", Section: sec, Label: f.Label + ": max taps", Min: 2, Max: 200, Step: 1, Default: f.Max},
			NumDef{Key: "spam." + f.Key + ".window", Section: sec, Label: f.Label + ": within", Unit: "s", Min: 1, Max: 30, Step: 0.5, Default: f.Window},
			NumDef{Key: "spam." + f.Key + ".lock", Section: sec, Label: f.Label + ": grumpy for", Unit: "s", Min: 1, Max: 30, Step: 0.5, Default: f.Lock},
		)
	}
}

func textDef(key string) (TextDef, bool) {
	i := slices.IndexFunc(textDefs, func(d TextDef) bool { return d.Key == key })
	if i < 0 {
		return TextDef{}, false
	}
	return textDefs[i], true
}

func listDef(key string) (ListDef, bool) {
	i := slices.IndexFunc(listDefs, func(d ListDef) bool { return d.Key == key })
	if i < 0 {
		return ListDef{}, false
	}
	return listDefs[i], true
}

func numDef(key string) (NumDef, bool) {
	i := slices.IndexFunc(numDefs, func(d NumDef) bool { return d.Key == key })
	if i < 0 {
		return NumDef{}, false
	}
	return numDefs[i], true
}

func flagDef(key string) (FlagDef, bool) {
	i := slices.IndexFunc(flagDefs, func(d FlagDef) bool { return d.Key == key })
	if i < 0 {
		return FlagDef{}, false
	}
	return flagDefs[i], true
}

func defaultConfig() *Config {
	c := &Config{
		Nickname:  "Mutu",
		BSMonth:   9, // Paush
		BSDay:     6,
		Timezone:  "Asia/Kathmandu",
		TeaseDays: 7,
	}
	c.normalize()
	return c
}

// normalize fills anything missing with defaults and clamps values, so older
// config files keep working when new fields are added.
func (c *Config) normalize() {
	if c.Overrides == nil {
		c.Overrides = map[int]string{}
	}
	if c.Flags == nil {
		c.Flags = map[string]bool{}
	}
	if c.Nums == nil {
		c.Nums = map[string]float64{}
	}
	if c.Texts == nil {
		c.Texts = map[string]string{}
	}
	if c.Lists == nil {
		c.Lists = map[string][]string{}
	}
	for _, d := range flagDefs {
		if _, ok := c.Flags[d.Key]; !ok {
			c.Flags[d.Key] = d.Default
		}
	}
	for _, d := range numDefs {
		v, ok := c.Nums[d.Key]
		if !ok {
			v = d.Default
		}
		c.Nums[d.Key] = min(max(v, d.Min), d.Max)
	}
	for _, d := range textDefs {
		if _, ok := c.Texts[d.Key]; !ok {
			c.Texts[d.Key] = d.Default
		}
	}
	for _, d := range listDefs {
		if l, ok := c.Lists[d.Key]; !ok || l == nil {
			if ok {
				c.Lists[d.Key] = []string{}
			} else {
				c.Lists[d.Key] = slices.Clone(d.Default)
			}
		}
	}
	if c.Milestones == nil {
		c.Milestones = slices.Clone(defaultMilestones)
	}
	slices.SortStableFunc(c.Milestones, func(a, b Milestone) int { return a.Count - b.Count })
	if c.Photos == nil {
		c.Photos = []Photo{}
	}
	if c.BSMonth < 1 || c.BSMonth > 12 {
		c.BSMonth = 9
	}
	if c.BSDay < 1 || c.BSDay > 32 {
		c.BSDay = 6
	}
	if c.Timezone == "" {
		c.Timezone = "Asia/Kathmandu"
	}
	c.TeaseDays = min(max(c.TeaseDays, 1), 60)
	switch c.ForceState {
	case "", "pre", "during", "post":
	default:
		c.ForceState = ""
	}
}

func (c *Config) clone() *Config {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	var out Config
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return &out
}

func (c *Config) location() *time.Location {
	if loc, err := time.LoadLocation(c.Timezone); err == nil {
		return loc
	}
	return time.FixedZone("NPT", 5*3600+45*60)
}

// Missing lists required fields that are empty or invalid. The setup wizard
// walks through these and every /config tab shows a banner while any remain.
type MissingField struct {
	Label, Tab, Step string
}

func (c *Config) Missing(now time.Time) []MissingField {
	var out []MissingField
	add := func(label, tab, step string) { out = append(out, MissingField{label, tab, step}) }
	if strings.TrimSpace(c.Nickname) == "" {
		add("Her nickname", "birthday", "1")
	}
	if strings.TrimSpace(c.SignOff) == "" {
		add("Your name (to sign the letter)", "birthday", "1")
	}
	if c.BirthBSYear == 0 {
		add("Her birth year (BS)", "birthday", "1")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		add("A valid timezone", "birthday", "2")
	}
	for _, d := range textDefs {
		if d.Required && strings.TrimSpace(c.Texts[d.Key]) == "" {
			add(d.Label, "text", "3")
		}
	}
	for _, d := range listDefs {
		if n := len(c.Lists[d.Key]); n < d.Min {
			add(fmt.Sprintf("%s (at least %d, have %d)", d.Label, d.Min, n), "text", "4")
		}
	}
	return out
}

// Validation helpers shared by the wizard and the tabs.

func validName(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("can't be empty")
	}
	if utf8.RuneCountInString(s) > 40 {
		return errors.New("keep it under 40 characters")
	}
	return nil
}

func validBirthYear(y int, now time.Time) error {
	cur := adToBSYear(now)
	if y < 2000 || y > cur-1 {
		return fmt.Errorf("should be a BS year between 2000 and %d (e.g. 2058)", cur-1)
	}
	return nil
}

// ConfigStore keeps the config in memory and persists every change atomically.
type ConfigStore struct {
	mu   sync.RWMutex
	path string
	cfg  *Config
}

func loadConfig(path string) (*ConfigStore, error) {
	s := &ConfigStore{path: path, cfg: defaultConfig()}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c.normalize()
	s.cfg = &c
	return s, nil
}

// Get returns a private copy that callers may read freely.
func (s *ConfigStore) Get() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.clone()
}

// Update applies fn to a copy, normalizes and saves it, then swaps it in.
// If fn returns an error nothing changes.
func (s *ConfigStore) Update(fn func(c *Config) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.cfg.clone()
	if err := fn(c); err != nil {
		return err
	}
	c.normalize()
	if err := writeJSONAtomic(s.path, c); err != nil {
		return err
	}
	s.cfg = c
	return nil
}

func writeJSONAtomic(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
