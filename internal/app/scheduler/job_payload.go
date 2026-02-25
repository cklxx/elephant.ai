package scheduler

import (
	"encoding/json"
	"fmt"
)

type jobPayload struct {
	Channel string `json:"channel,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	ChatID  string `json:"chat_id,omitempty"`
	GoalID  string `json:"goal_id,omitempty"`
}

func payloadFromTrigger(trigger Trigger) (json.RawMessage, error) {
	payload := jobPayload{
		Channel: trigger.Channel,
		UserID:  trigger.UserID,
		ChatID:  trigger.ChatID,
		GoalID:  trigger.GoalID,
	}
	if payload == (jobPayload{}) {
		return nil, nil
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("job payload: marshal failed: %w", err)
	}
	return data, nil
}

func triggerFromJob(job Job) (Trigger, error) {
	payload := jobPayload{}
	if len(job.Payload) > 0 {
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return Trigger{}, fmt.Errorf("job payload: unmarshal failed: %w", err)
		}
	}
	return Trigger{
		Name:     job.ID,
		Schedule: job.CronExpr,
		Task:     job.Trigger,
		Channel:  payload.Channel,
		UserID:   payload.UserID,
		ChatID:   payload.ChatID,
		GoalID:   payload.GoalID,
	}, nil
}
