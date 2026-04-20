package drawful

import (
	"fmt"
	"math/rand"
	"sync"
)

const (
	generatedPromptCount = 10000
	generatedPoolSize    = 1000
)

// DefaultPrompts is the embedded Jrawful dictionary. It starts with the
// hand-written prompts below, then appends 1000 deterministic generated
// combinations so long games, rerolls, and generated fake choices have plenty
// of variety without needing an external asset file.
var DefaultPrompts = buildDefaultPrompts()

var basePrompts = []string{
	"a pirate walking a dog",
	"a ghost ordering coffee",
	"a cat in a tiny hat",
	"the last slice of pizza",
	"a robot doing laundry",
	"a werewolf on a first date",
	"a bear playing cello",
	"a wizard losing a phone",
	"a knight afraid of a duck",
	"an astronaut mowing the lawn",
	"a raccoon with a briefcase",
	"a t-rex with tiny chopsticks",
	"a vampire at a yoga class",
	"a mummy riding a skateboard",
	"a detective in pajamas",
	"a ninja making toast",
	"a cowboy using a selfie stick",
	"a chef yelling at a salad",
	"a dragon booking a flight",
	"a grandma taming a tiger",
	"a penguin stuck in a vending machine",
	"a wizard calling tech support",
	"a superhero doing their taxes",
	"an octopus playing eight guitars",
	"a snowman on vacation",
	"a unicorn eating cereal",
	"a shark on a motorcycle",
	"a gingerbread man escaping",
	"a mermaid doing karaoke",
	"a zombie throwing a dinner party",
	"a medieval knight with an umbrella",
	"a squid reading a newspaper",
	"a pirate stuck in IKEA",
	"a scientist walking a dinosaur",
	"a panda as a traffic cop",
	"a sumo wrestler doing ballet",
	"a mailman delivering to the moon",
	"a fortune teller predicting rain",
	"a bride chasing a pigeon",
	"a gorilla playing chess",
	"a librarian on a roller coaster",
	"a grandmother arm-wrestling a bear",
	"a vampire accountant",
	"a farmer harvesting spaghetti",
	"a clown hiding from a mime",
	"a snail in a hurry",
	"a surfer riding a bathtub",
	"a barber cutting a llama",
	"a chef cooking a cloud",
	"a king stuck in a revolving door",
	"a spy pretending to be a statue",
	"a cactus going to the beach",
	"a pigeon reviewing a restaurant",
	"a sheep as a DJ",
	"a hamster lifting weights",
	"a toddler teaching a class",
	"an alien tipping a waiter",
	"a bigfoot at a wedding",
	"a pigeon in a suit interviewing",
	"a kraken in a kiddie pool",
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
	twists := generatedTwists()

	out := make([]string, 0, generatedPromptCount)
	for i := 0; i < generatedPromptCount; i++ {
		round := i / generatedPoolSize
		subject := subjects[i%generatedPoolSize]
		action := actions[(i*37+round)%generatedPoolSize]
		twist := twists[(i*101+round*17)%generatedPoolSize]
		out = append(out, fmt.Sprintf("%s %s %s", subject, action, twist))
	}
	return out
}

func generatedSubjects() []string {
	descriptors := []string{
		"sleepy", "tiny", "giant", "fancy", "muddy", "glowing", "grumpy", "cheerful", "sneaky", "sparkly",
		"round", "wobbly", "brave", "lost", "dramatic", "polka-dot", "striped", "cardboard", "royal", "rainbow",
		"hairy", "frozen", "melty", "noisy", "shy", "ancient", "baby", "clockwork", "floating", "dizzy",
		"fuzzy", "golden", "tiny-hat", "roller-skating", "backwards", "overdressed", "undercover", "pajama", "balloon", "mustached",
	}
	nouns := []string{
		"baker", "robot", "dragon", "ghost", "raccoon", "penguin", "pirate", "wizard", "cowboy", "astronaut",
		"vampire", "detective", "knight", "mermaid", "alien", "hamster", "octopus", "llama", "shark", "unicorn",
		"chef", "clown", "bear", "pigeon", "frog",
	}
	return combineTwo("the %s %s", descriptors, nouns)
}

func generatedActions() []string {
	verbs := []string{
		"juggling", "painting", "chasing", "hugging", "building", "riding", "washing", "wearing", "carrying", "kicking",
		"throwing", "catching", "selling", "buying", "wrapping", "opening", "hiding inside", "dancing with", "singing to", "arguing with",
		"cooking", "planting", "stacking", "balancing", "fixing", "stealing", "guarding", "teaching", "photographing", "mailing",
		"brushing", "rescuing", "inflating", "deflating", "decorating", "measuring", "polishing", "surfing on", "sleeping on", "walking",
	}
	objects := []string{
		"a giant cupcake", "a tiny car", "a beach ball", "a rubber duck", "a pizza slice", "a birthday cake", "a treasure chest", "a magic broom", "a traffic cone", "a moon rock",
		"a skateboard", "a shopping cart", "a fish bowl", "a cactus", "a snow globe", "a telescope", "a banana peel", "a suitcase", "a toaster", "a teddy bear",
		"a ladder", "a bubble wand", "a sandwich", "a kite", "a garden hose",
	}
	return combineTwo("%s %s", verbs, objects)
}

func generatedTwists() []string {
	scenes := []string{
		"at a beach picnic", "during a rainstorm", "inside a treehouse", "on a rooftop", "in a grocery store", "at a school dance", "inside a haunted house", "on a pirate ship", "at a space station", "in a castle kitchen",
		"at a carnival booth", "inside a snow fort", "on a farm", "in a movie theater", "at a tiny airport", "inside a submarine", "at a dragon parade", "in a messy bedroom", "on a mountain trail", "at a roller rink",
		"in a library", "at a bowling alley", "inside a giant shoe", "at a swimming pool", "on a cloud",
		"at a dog show", "inside a toy store", "on a school bus", "at a campfire", "in a pumpkin patch", "at a science fair", "inside a clock tower", "on a fishing dock", "at a fancy wedding", "in a candy factory",
		"at a circus tent", "inside a blanket cave", "on a mini golf course", "at a noodle shop", "in a flower garden",
	}
	props := []string{
		"with a red umbrella", "beside a stack of pancakes", "under a disco ball", "next to a sleeping cat", "with balloons everywhere", "near a broken clock", "with a crown on top", "beside a puddle", "under a spotlight", "next to a warning sign",
		"with confetti falling", "beside a tiny door", "with a map upside down", "near a pile of socks", "with soap bubbles", "beside a giant spoon", "under a rainbow", "next to a squeaky toy", "with stars painted on it", "beside a mystery box",
		"wearing sunglasses", "with a trail of footprints", "next to a lemonade stand", "under a blanket fort", "with a trophy nearby",
	}
	return combineTwo("%s %s", scenes, props)
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
