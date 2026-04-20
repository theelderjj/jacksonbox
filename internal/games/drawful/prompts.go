package drawful

import (
	"fmt"
	"math/rand"
	"sync"
)

const (
	generatedPromptCount = 50000
	generatedPoolSize    = 5000
)

// DefaultPrompts is the embedded Jrawful dictionary. It starts with the
// hand-written prompts below, then appends deterministic generated prompts so
// long games, rerolls, and generated fake choices have plenty of variety
// without needing an external asset file.
var DefaultPrompts = buildDefaultPrompts()

var basePrompts = []string{
	"a pirate dancing",
	"a ghost ordering coffee",
	"a cat in a tiny hat",
	"a robot doing laundry",
	"a werewolf on a first date",
	"a bear playing cello",
	"a wizard panicking",
	"a knight hiding",
	"an astronaut mowing",
	"a raccoon sneaking",
	"a t-rex eating",
	"a vampire at yoga",
	"a mummy skateboarding",
	"a detective in pajamas",
	"a ninja making toast",
	"a cowboy taking selfies",
	"a chef yelling",
	"a dragon flying",
	"a grandma wrestling",
	"a penguin stuck",
	"a superhero doing taxes",
	"an octopus playing guitar",
	"a snowman on vacation",
	"a unicorn eating cereal",
	"a shark riding",
	"a gingerbread man escaping",
	"a mermaid doing karaoke",
	"a zombie hosting dinner",
	"a squid reading",
	"a scientist with a dinosaur",
	"a panda directing traffic",
	"a sumo wrestler doing ballet",
	"a fortune teller predicting rain",
	"a bride chasing pigeons",
	"a gorilla playing chess",
	"a librarian on a roller coaster",
	"a vampire accountant",
	"a farmer harvesting spaghetti",
	"a clown hiding",
	"a snail racing",
	"a cactus at the beach",
	"a pigeon reviewing food",
	"a sheep DJing",
	"a hamster lifting weights",
	"a toddler teaching",
	"an alien tipping",
	"bigfoot at a wedding",
	"a kraken in a pool",
}

func buildDefaultPrompts() []string {
	prompts := make([]string, 0, len(basePrompts)+generatedPromptCount)
	prompts = append(prompts, basePrompts...)
	prompts = append(prompts, generatedPrompts()...)
	return prompts
}

func generatedPrompts() []string {
	subjects := generatedSubjects()
	actions := generatedActions()
	scenes := generatedScenes()

	out := make([]string, 0, generatedPromptCount)
	for i := 0; i < generatedPromptCount; i++ {
		round := i / generatedPoolSize
		subject := subjects[i%generatedPoolSize]
		if i%2 == 0 {
			action := actions[(i*37+round)%generatedPoolSize]
			out = append(out, fmt.Sprintf("%s %s", subject, action))
			continue
		}
		scene := scenes[(i*101+round*17)%generatedPoolSize]
		out = append(out, fmt.Sprintf("%s %s", subject, scene))
	}
	return out
}

func generatedSubjects() []string {
	descriptors := []string{
		"sleepy", "tiny", "giant", "fancy", "muddy", "glowing", "grumpy", "cheerful", "sneaky", "sparkly",
		"round", "wobbly", "brave", "lost", "dramatic", "polka-dot", "striped", "cardboard", "royal", "rainbow",
		"hairy", "frozen", "melty", "noisy", "shy", "ancient", "baby", "clockwork", "floating", "dizzy",
		"fuzzy", "golden", "tiny-hat", "roller-skating", "backwards", "overdressed", "undercover", "pajama", "balloon", "mustached",
		"heroic", "villainous", "cosmic", "rubbery", "pixel", "neon", "sleepwalking", "sweaty", "famous", "invisible",
		"confused", "square", "spiky", "soggy", "windy", "plastic", "paper", "velvet", "thunder", "electric",
		"mini", "mega", "wizardly", "cowboy", "ninja", "pirate", "space", "beach", "school", "garage",
		"bubble", "slimy", "chunky", "noodle", "marshmallow", "pickle", "waffle", "laser", "comic-book", "cartoon",
		"retro", "y2k", "sketchy", "clumsy", "speedy", "sleep-deprived", "dramatic-cape", "helmet", "sock-puppet", "glitter",
		"moon", "sunburned", "snowy", "rainy", "sticky", "squishy", "gravy", "chrome", "garage-band", "paper-crown",
	}
	nouns := []string{
		"SpongeBob", "Patrick Star", "Squidward", "Sandy Cheeks", "Timmy Turner",
		"Cosmo", "Wanda", "Jimmy Neutron", "Danny Phantom", "Jenny Wakeman",
		"Aang", "CatDog", "Arnold", "Helga", "Tommy Pickles",
		"Dexter", "Dee Dee", "Johnny Bravo", "Courage", "Ed Edd n Eddy",
		"Samurai Jack", "Ben Tennyson", "Blossom", "Bubbles", "Buttercup",
		"Raven", "Starfire", "Cyborg", "Beast Boy", "Grim",
		"Spider-Man", "Wolverine", "Hulk", "Iron Man", "Captain America",
		"Thor", "Black Panther", "Storm", "Cyclops", "Deadpool",
		"Batman", "Superman", "Wonder Woman", "The Flash", "Green Lantern",
		"Aquaman", "Robin", "Batgirl", "Static Shock", "Martian Manhunter",
	}
	return combineTwo("the %s %s", descriptors, nouns)
}

func generatedActions() []string {
	verbs := []string{
		"dancing", "running", "jumping", "sneaking", "flying", "falling", "spinning", "posing", "waving", "crying",
		"laughing", "sleeping", "skating", "surfing", "swimming", "climbing", "hiding", "floating", "shrinking", "growing",
		"singing", "rapping", "drumming", "reading", "drawing", "painting", "cooking", "cleaning", "digging", "fishing",
		"boxing", "stretching", "lifting", "marching", "tiptoeing", "moonwalking", "teleporting", "glitching", "melting", "freezing",
		"cartwheeling", "breakdancing", "sprinting", "tripping", "sliding", "bouncing", "hovering", "burping", "screaming", "whispering",
		"cheering", "daydreaming", "panicking", "meditating", "flexing", "bowling", "juggling", "gardening", "baking", "typing",
		"texting", "photobombing", "grimacing", "pranking", "sledding", "snowboarding", "rollerblading", "paddling", "diving", "camping",
		"parading", "conducting", "sweeping", "mopping", "building", "repairing", "inventing", "investigating", "detecting", "patrolling",
		"rescuing", "guarding", "chasing", "dodging", "catching", "throwing", "kicking", "punching", "flipping", "stomping",
		"levitating", "glowing", "elastic-stretching", "transforming", "time-traveling", "shape-shifting", "web-swinging", "power-posing", "cape-flapping", "laser-pointing",
	}
	moods := []string{
		"happily", "angrily", "nervously", "dramatically", "slowly", "quickly", "badly", "proudly", "secretly", "loudly",
		"quietly", "awkwardly", "bravely", "sleepily", "wildly", "carefully", "backwards", "sideways", "underwater", "upside down",
		"on stage", "in disguise", "in pajamas", "in boots", "with style",
		"with jazz hands", "like a robot", "like a crab", "like a zombie", "like a superhero", "like a villain", "during lunch", "after school", "before bedtime", "at midnight",
		"in slow motion", "with confidence", "with panic", "without blinking", "while floating", "while sparkling", "while dizzy", "while tiny", "while giant", "while invisible",
		"for applause", "for a trophy", "for no reason", "on one foot", "with attitude",
	}
	return combineTwo("%s %s", verbs, moods)
}

func generatedScenes() []string {
	places := []string{
		"school", "beach", "space", "rooftop", "grocery store", "school dance", "haunted house", "pirate ship", "castle", "carnival",
		"snow fort", "farm", "movie theater", "airport", "submarine", "bedroom", "mountain", "roller rink", "library", "bowling alley",
		"pool", "cloud", "dog show", "toy store", "school bus", "campfire", "pumpkin patch", "science fair", "clock tower", "dock",
		"wedding", "candy factory", "circus", "blanket fort", "mini golf", "noodle shop", "garden", "sewer", "mall", "comic shop",
		"arcade", "treehouse", "cafeteria", "playground", "skate park", "moon base", "secret lab", "water park", "pizza shop", "video store",
	}
	styles := []string{
		"spooky", "sunny", "rainy", "snowy", "messy", "tiny", "giant", "underwater", "glowing", "haunted",
		"fancy", "gross", "secret", "loud", "quiet", "upside-down", "rainbow", "muddy", "space", "cartoon",
		"school", "birthday", "midnight", "summer", "winter",
		"neon", "retro", "y2k", "comic-book", "superhero", "villain", "slimy", "sparkly", "foggy", "windy",
		"electric", "laser", "moonlit", "sunset", "stormy", "frozen", "melty", "rubbery", "cardboard", "paper",
		"plastic", "golden", "silver", "candy", "pizza", "noodle", "waffle", "pickle", "bubble", "banana",
		"dinosaur", "robot", "pirate", "wizard", "ninja", "cowboy", "alien", "monster", "ghost", "vampire",
		"zombie", "dragon", "undercover", "dramatic", "sleepy", "grumpy", "cheerful", "clumsy", "speedy", "slow-motion",
		"sideways", "backwards", "floating", "invisible", "miniature", "oversized", "paint-splattered", "confetti", "chalk", "crayon",
		"marker", "sticker", "garage-band", "field-trip", "detention", "recess", "karaoke", "talent-show", "snow-day", "summer-camp",
		"hairstyle", "helmet", "cape", "mask", "sock-puppet",
	}
	return combineTwo("at the %s %s", styles, places)
}

func combineTwo(format string, left []string, right []string) []string {
	out := make([]string, 0, len(left)*len(right))
	for _, l := range left {
		for _, r := range right {
			out = append(out, fmt.Sprintf(format, l, r))
		}
	}
	return out
}

// RandomPrompts returns n unique prompts from the dictionary. Deterministic
// given the rng. Panics if n exceeds the dictionary size — caller's bug.
func RandomPrompts(r *rand.Rand, n int) []string {
	if n > len(DefaultPrompts) {
		panic("drawful: asked for more prompts than the dictionary holds")
	}
	shuffled := make([]string, len(DefaultPrompts))
	copy(shuffled, DefaultPrompts)
	r.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled[:n]
}

var rngMu sync.Mutex
var sharedRng = rand.New(rand.NewSource(1))

// Seed resets the prompt rng. Test helper.
func Seed(seed int64) {
	rngMu.Lock()
	defer rngMu.Unlock()
	sharedRng = rand.New(rand.NewSource(seed))
}

// NextPrompts issues n unique prompts using the shared rng.
func NextPrompts(n int) []string {
	rngMu.Lock()
	defer rngMu.Unlock()
	return RandomPrompts(sharedRng, n)
}
