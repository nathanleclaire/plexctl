# plexctl

A no-frills, Go-based CLI for the Perplexity API.

## Installation

```bash
go install github.com/nathanleclaire/plexctl/cmd/plexctl@latest
```

## Configuration

Set your Perplexity API token:

```bash
export PERPLEXITY_API_TOKEN=your-token-here
```

Or pass it directly:

```bash
plexctl --token=your-token-here [command]
```

## Commands

### Get a Completion

Generate a quick response to any query:

```bash
$ plexctl get "Write the most San Francisco sentence ever"
As the fog rolled in off the Pacific, a cable car rattled up Nob Hill, passing by the historic mansions of the robber barons, while a group of tech entrepreneurs sipped coffee in a trendy café in the Mission, discussing their latest startup idea, all just a stone's throw from the vibrant streets of Chinatown and the iconic Golden Gate Bridge.
```

### Manage Threads

List your conversation threads:

```bash
$ plexctl thread
THREAD ID  FIRST USER MESSAGE
GpunfKmq   write the most bay area sentence possible
9Pme8Nvc   what's a cool dinosaur i never heard of?
5TGFAeda   tell me a yo momma joke
FxjcGToh   hell world
9PFRvf5U   what are server side events?
```

View a specific thread:

```bash
$ plexctl thread get GpunfKmq
THREAD: GpunfKmq

[0] USER:
write the most bay area sentence possible

[1] ASSISTANT:
Here's a sentence that captures the essence of the Bay Area:

"As I sipped a pour-over coffee at a trendy cafe in Haight-Ashbury, I pondered the tech startup I was about to pitch in Silicon Valley, while simultaneously worrying about the housing market and the next Warriors game."

This sentence incorporates several quintessential Bay Area elements:
- **Haight-Ashbury**: Known for its historical significance in the 1960s counterculture movement.
- **Pour-over coffee**: Reflects the Bay Area's vibrant coffee culture.
- **Tech startup**: References the region's status as a hub for technology and innovation.
- **Silicon Valley**: The heart of the tech industry.
- **Housing market**: Acknowledges the area's notoriously high cost of living.
- **Warriors game**: Mentions the Golden State Warriors, a beloved local sports team.
```

## Options

```bash
--debug          Enable debug logs to stderr
--token string   Perplexity API token (env PERPLEXITY_API_TOKEN)
```

## Examples

Generate amusing AI content:

```bash
$ plexctl get "Write a haiku about debugging Go code"
Nil pointers hide
Goroutines sleep at dawn
Context canceled
```

Get concise answers to technical questions:

```bash
$ plexctl get "What's the difference between channels and mutexes in Go?"
Channels are for communication between goroutines, enabling safe data passing with built-in synchronization, while mutexes control access to shared resources by allowing only one goroutine to access the protected section at a time.
```

Have fun with creative prompts:

```bash
$ plexctl get "Write the most tech bro sentence ever"
Just crushed a 5-hour Zoom marathon from my WeWork hot desk, optimized my biohacking stack with nootropics from my YC-backed subscription box, then closed a $10M seed round for my Web3 SaaS platform that's basically Uber for NFTs, all before my afternoon Peloton session and ceremonial microdose.
```
