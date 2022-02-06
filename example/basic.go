package main

import (
    "log"
    "meow.tf/websub/client"
    "net/http"
)

func main() {
    c := client.New("https://YOUR_PUBLIC_URL")

    // Listen on port 8080 for the client
    go http.ListenAndServe(":8080", c)

    // When you use SubscribeOptions, you can either:
    // Generate a callback yourself
    // OR let the client generate one (using the sha256 sum of the topic)
    sub, err := c.Subscribe(client.SubscribeOptions{
        Topic: "https://websub.rocks/blog/301",
        Secret: "testing123",
    })

    if err != nil {
        log.Fatalln(err)
    }

    log.Println("Successfully subscribed! Lease:", sub.LeaseTime)

    <- make(chan struct{})
}