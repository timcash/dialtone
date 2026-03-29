package repl

import "flag"

func topicFlag(fs *flag.FlagSet, usage string) *string {
	topic := defaultRoom
	fs.StringVar(&topic, "topic", defaultRoom, usage)
	fs.StringVar(&topic, "room", defaultRoom, "Legacy alias for --topic")
	return &topic
}
