package activity

type EventType string

const (
	EventSpawned      EventType = "spawned"
	EventHeartbeat    EventType = "heartbeat"
	EventMailSent     EventType = "mail_sent"
	EventMailReceived EventType = "mail_received"
	EventMailNudged   EventType = "mail_nudged"
	EventRunning      EventType = "running"
	EventWaiting      EventType = "waiting"
	EventStuck        EventType = "stuck"
	EventCompleted    EventType = "completed"
	EventError        EventType = "error"
	EventClosed       EventType = "closed"
)
