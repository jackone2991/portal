// Command opsenqueue is a throwaway dev helper: it enqueues one
// ops:backup_database task so the restore drill can be exercised on demand
// (the real task fires nightly on the scheduler). Not part of the product.
package main

import (
	"log"

	"github.com/hibiken/asynq"
)

func main() {
	c := asynq.NewClient(asynq.RedisClientOpt{Addr: "dragonfly:6379", DB: 1})
	defer c.Close()
	info, err := c.Enqueue(asynq.NewTask("ops:backup_database", nil, asynq.Queue("default")))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("enqueued ops:backup_database id=%s queue=%s", info.ID, info.Queue)
}
