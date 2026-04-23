package wordstorm

var dictionary = func() map[string]bool {
	words := map[string]bool{}
	for _, word := range []string{
		"apple", "apricot", "avocado", "anchor", "animal", "artist", "autumn", "arrow",
		"bacon", "bagel", "banana", "basket", "beacon", "beans", "beaver", "biscuit", "blanket", "bottle", "bread", "bridge", "broccoli", "bubble", "button",
		"cabin", "cabinet", "camel", "camera", "candle", "canary", "canyon", "carpet", "castle", "cat", "caterpillar", "cheetah", "chicken", "chimney", "coconut", "cobra", "cookie", "coyote", "cricket", "crocodile",
		"daisy", "dancer", "dragon", "drawer", "dream", "driver", "drizzle", "dolphin", "donkey", "diamond",
		"eagle", "earth", "ember", "engine", "evening", "expert", "echo", "elbow", "energy",
		"fabric", "falcon", "family", "farmer", "feather", "fishing", "flower", "forest", "fork", "friend", "fridge",
		"garden", "garlic", "giraffe", "glacier", "goblin", "guitar", "hammer", "harbor", "helmet", "honey", "house", "hunter",
		"island", "jacket", "jungle", "kitten", "ladder", "lantern", "lemon", "library", "lion", "lizard",
		"magnet", "mirror", "monkey", "mountain", "music", "napkin", "needle", "ocean", "orange", "painter", "pencil", "pillow", "planet", "pocket", "pumpkin",
		"rabbit", "rainbow", "remote", "river", "robot", "rocket", "saddle", "school", "shadow", "shark", "shell", "shirt", "silver", "skating", "snake", "snow", "spoon", "stream", "sunset",
		"table", "tiger", "towel", "train", "turtle", "valley", "velvet", "violin", "wallet", "window", "winter", "wizard", "yellow", "zebra",
	} {
		words[word] = true
	}
	return words
}()
