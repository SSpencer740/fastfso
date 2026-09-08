package email

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	taskspb "cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
)

// TaskCreator abstracts the Cloud Tasks client for testing.
type TaskCreator interface {
	CreateTask(ctx context.Context, req *taskspb.CreateTaskRequest, opts ...interface{}) (*taskspb.Task, error)
}

// QueueSender enqueues emails as Cloud Tasks HTTP requests.
type QueueSender struct {
	client    *cloudtasks.Client
	queue     string
	targetURL string
	audience  string
	saEmail   string
	logger    *slog.Logger
}

// NewQueueSender creates a QueueSender.
func NewQueueSender(client *cloudtasks.Client, queue, backendURL, saEmail string, logger *slog.Logger) *QueueSender {
	return &QueueSender{
		client:    client,
		queue:     queue,
		targetURL: backendURL + "/api/tasks/send-email",
		audience:  backendURL,
		saEmail:   saEmail,
		logger:    logger,
	}
}

// Send enqueues an email message as a Cloud Task.
func (q *QueueSender) Send(ctx context.Context, msg Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal email message: %w", err)
	}

	req := &taskspb.CreateTaskRequest{
		Parent: q.queue,
		Task: &taskspb.Task{
			MessageType: &taskspb.Task_HttpRequest{
				HttpRequest: &taskspb.HttpRequest{
					HttpMethod: taskspb.HttpMethod_POST,
					Url:        q.targetURL,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       body,
					AuthorizationHeader: &taskspb.HttpRequest_OidcToken{
						OidcToken: &taskspb.OidcToken{
							ServiceAccountEmail: q.saEmail,
							Audience:            q.audience,
						},
					},
				},
			},
		},
	}

	task, err := q.client.CreateTask(ctx, req)
	if err != nil {
		return fmt.Errorf("create cloud task: %w", err)
	}

	q.logger.InfoContext(ctx, "email task enqueued", "to", msg.To, "subject", msg.Subject, "task", task.Name)
	return nil
}
