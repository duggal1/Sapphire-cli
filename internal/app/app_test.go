package app

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"github.com/duggal1/Sapphire-cli/internal/agent"
	agentbackground "github.com/duggal1/Sapphire-cli/internal/agent/background"
	agentformula "github.com/duggal1/Sapphire-cli/internal/agent/formula"
	"github.com/duggal1/Sapphire-cli/internal/agent/planmode"
	"github.com/duggal1/Sapphire-cli/internal/codeindex"
	"github.com/duggal1/Sapphire-cli/internal/config"
	"github.com/duggal1/Sapphire-cli/internal/message"
	orchestrationdb "github.com/duggal1/Sapphire-cli/internal/orchestration/db"
	"github.com/duggal1/Sapphire-cli/internal/permission"
	"github.com/duggal1/Sapphire-cli/internal/pubsub"
	"github.com/duggal1/Sapphire-cli/internal/session"
	"github.com/duggal1/Sapphire-cli/internal/worktreepolicy"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// TestSetupSubscriber_NormalFlow verifies that the subscriber correctly propagates
// published events from the broker to the output channel under normal conditions.
func TestSetupSubscriber_NormalFlow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 10, subscriberBestEffort)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		for range 2 {
			select {
			case <-f.outputCh:
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for messages")
			}
		}

		f.cancel()
		f.wg.Wait()
	})
}

// TestSetupSubscriber_SlowConsumer ensures that the subscriber correctly handles scenarios
// where the consumer is too slow by dropping messages rather than blocking indefinitely.
func TestSetupSubscriber_SlowConsumer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 0, subscriberBestEffort)

		const numEvents = 5

		var pubWg sync.WaitGroup
		pubWg.Go(func() {
			for range numEvents {
				f.broker.Publish(pubsub.CreatedEvent, "event")
				time.Sleep(10 * time.Millisecond)
				synctest.Wait()
			}
		})

		time.Sleep(time.Duration(numEvents) * (subscriberSendTimeout + 20*time.Millisecond))
		synctest.Wait()

		received := 0
		for {
			select {
			case <-f.outputCh:
				received++
			default:
				pubWg.Wait()
				f.cancel()
				f.wg.Wait()
				require.Less(t, received, numEvents, "Slow consumer should have dropped some messages")
				return
			}
		}
	})
}

// TestSetupSubscriber_ContextCancellation verifies that the subscriber goroutine
// terminates correctly when the provided context is cancelled.
func TestSetupSubscriber_ContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 10, subscriberBestEffort)

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(100 * time.Millisecond)
		synctest.Wait()

		f.cancel()
		f.wg.Wait()
	})
}

// TestSetupSubscriber_DrainAfterDrop tests the edge case where a message is dropped
// and ensures that subsequent events do not cause a deadlock in the timer drain logic.
func TestSetupSubscriber_DrainAfterDrop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 0, subscriberBestEffort)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		// First event: nobody reads outputCh so the timer fires (message dropped).
		f.broker.Publish(pubsub.CreatedEvent, "event1")
		time.Sleep(subscriberSendTimeout + 25*time.Millisecond)
		synctest.Wait()

		// Second event: triggers Stop()==false path; without the fix this deadlocks.
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		// If the timer drain deadlocks, wg.Wait never returns.
		done := make(chan struct{})
		go func() {
			f.cancel()
			f.wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("setupSubscriber goroutine hung — likely timer drain deadlock")
		}
	})
}

// TestSetupSubscriber_NoTimerLeak ensures that the subscriber does not leak
// timer resources during high-frequency event publishing and subsequent shutdown.
func TestSetupSubscriber_NoTimerLeak(t *testing.T) {
	defer goleak.VerifyNone(t)
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 100, subscriberBestEffort)

		for range 100 {
			f.broker.Publish(pubsub.CreatedEvent, "event")
			time.Sleep(5 * time.Millisecond)
			synctest.Wait()
		}

		f.cancel()
		f.wg.Wait()
	})
}

func TestSetupSubscriber_CriticalDoesNotDropMessages(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newSubscriberFixture(t, 2, subscriberCritical)

		time.Sleep(10 * time.Millisecond)
		synctest.Wait()

		f.broker.Publish(pubsub.CreatedEvent, "event1")
		f.broker.Publish(pubsub.CreatedEvent, "event2")

		for range 2 {
			select {
			case <-f.outputCh:
			case <-time.After(5 * time.Second):
				t.Fatal("Timed out waiting for critical message")
			}
		}

		f.cancel()
		f.wg.Wait()
	})
}

func TestIsNonInteractiveRuntime(t *testing.T) {
	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "")
	require.False(t, isNonInteractiveRuntime())

	t.Setenv("SAPPHIRE_NON_INTERACTIVE", "1")
	require.True(t, isNonInteractiveRuntime())
}

type sessionServiceStub struct {
	nextID int
}

func (s *sessionServiceStub) Subscribe(context.Context) <-chan pubsub.Event[session.Session] {
	return nil
}

func (s *sessionServiceStub) Create(context.Context, string) (session.Session, error) {
	s.nextID++
	return session.Session{ID: "session-test"}, nil
}

func (s *sessionServiceStub) CreateTitleSession(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}

func (s *sessionServiceStub) CreateTaskSession(context.Context, string, string, string) (session.Session, error) {
	return session.Session{}, nil
}

func (s *sessionServiceStub) Get(context.Context, string) (session.Session, error) {
	return session.Session{}, nil
}
func (s *sessionServiceStub) List(context.Context) ([]session.Session, error) { return nil, nil }
func (s *sessionServiceStub) Save(context.Context, session.Session) (session.Session, error) {
	return session.Session{}, nil
}
func (s *sessionServiceStub) UpdateTitleAndUsage(context.Context, string, string, int64, int64, float64) error {
	return nil
}
func (s *sessionServiceStub) Delete(context.Context, string) error { return nil }
func (s *sessionServiceStub) CreateAgentToolSessionID(messageID, toolCallID string) string {
	return messageID + ":" + toolCallID
}
func (s *sessionServiceStub) ParseAgentToolSessionID(sessionID string) (string, string, bool) {
	return "", "", false
}
func (s *sessionServiceStub) IsAgentToolSession(string) bool { return false }
func (s *sessionServiceStub) SetMode(context.Context, string, planmode.SessionMode) error {
	return nil
}
func (s *sessionServiceStub) GetMode(context.Context, string) (planmode.SessionMode, error) {
	return "", nil
}
func (s *sessionServiceStub) SetWorktreePolicy(context.Context, string, worktreepolicy.Policy) error {
	return nil
}
func (s *sessionServiceStub) GetWorktreePolicy(context.Context, string) (worktreepolicy.Policy, error) {
	return "", nil
}

type messageServiceStub struct{}

func (s *messageServiceStub) Subscribe(context.Context) <-chan pubsub.Event[message.Message] {
	return nil
}

func (s *messageServiceStub) Create(context.Context, string, message.CreateMessageParams) (message.Message, error) {
	return message.Message{}, nil
}
func (s *messageServiceStub) Update(context.Context, message.Message) error { return nil }
func (s *messageServiceStub) Get(context.Context, string) (message.Message, error) {
	return message.Message{}, nil
}
func (s *messageServiceStub) List(context.Context, string) ([]message.Message, error) {
	return nil, nil
}
func (s *messageServiceStub) ListUserMessages(context.Context, string) ([]message.Message, error) {
	return nil, nil
}
func (s *messageServiceStub) ListAllUserMessages(context.Context) ([]message.Message, error) {
	return nil, nil
}
func (s *messageServiceStub) Delete(context.Context, string) error                { return nil }
func (s *messageServiceStub) DeleteSessionMessages(context.Context, string) error { return nil }

type runOnlyCoordinatorStub struct{}

func (s *runOnlyCoordinatorStub) Run(context.Context, string, string, ...message.Attachment) (*fantasy.AgentResult, error) {
	return &fantasy.AgentResult{}, nil
}
func (s *runOnlyCoordinatorStub) Submit(context.Context, string, string, ...message.Attachment) (agent.SubmissionResult, error) {
	return agent.SubmissionResult{}, nil
}
func (s *runOnlyCoordinatorStub) OrchestrateWorktrees(context.Context, string, agent.OrchestrateWorktreesParams) (agent.OrchestrateWorktreesResult, error) {
	return agent.OrchestrateWorktreesResult{}, nil
}
func (s *runOnlyCoordinatorStub) ResumeWorktree(context.Context, string, string, string, string, string, string) (agent.OrchestrationAgentRef, error) {
	return agent.OrchestrationAgentRef{}, nil
}
func (s *runOnlyCoordinatorStub) Cancel(string)             {}
func (s *runOnlyCoordinatorStub) CancelAll()                {}
func (s *runOnlyCoordinatorStub) IsSessionBusy(string) bool { return false }
func (s *runOnlyCoordinatorStub) IsBusy() bool              { return false }
func (s *runOnlyCoordinatorStub) QueuedPrompts(string) int  { return 0 }
func (s *runOnlyCoordinatorStub) QueuedPromptsList(string) []string {
	return nil
}
func (s *runOnlyCoordinatorStub) ClearQueue(string) {}
func (s *runOnlyCoordinatorStub) Summarize(context.Context, string) error {
	return nil
}
func (s *runOnlyCoordinatorStub) Model() agent.Model { return agent.Model{} }
func (s *runOnlyCoordinatorStub) UpdateModels(context.Context) error {
	return nil
}
func (s *runOnlyCoordinatorStub) MemoryPipe() interface{} { return nil }
func (s *runOnlyCoordinatorStub) ConsolidateMemory(context.Context, string) error {
	return nil
}
func (s *runOnlyCoordinatorStub) DispatchBackground(context.Context, agentbackground.TaskSpec) (string, error) {
	return "", nil
}
func (s *runOnlyCoordinatorStub) GetBackgroundStatus(string) (agentbackground.SubAgent, bool) {
	return agentbackground.SubAgent{}, false
}
func (s *runOnlyCoordinatorStub) ListBackgroundAgents() []agentbackground.SubAgent { return nil }
func (s *runOnlyCoordinatorStub) WaitForCompletion(context.Context, []string) ([]agentbackground.SubAgent, error) {
	return nil, nil
}
func (s *runOnlyCoordinatorStub) IndexCodebase(context.Context, bool) (codeindex.Stats, error) {
	return codeindex.Stats{}, nil
}
func (s *runOnlyCoordinatorStub) ListWorktrees(context.Context, string, []string, int) ([]orchestrationdb.WorktreeRun, error) {
	return nil, nil
}
func (s *runOnlyCoordinatorStub) LandWorktree(context.Context, string, string) (orchestrationdb.WorktreeRun, error) {
	return orchestrationdb.WorktreeRun{}, nil
}
func (s *runOnlyCoordinatorStub) RepairWorktree(context.Context, string) (orchestrationdb.WorktreeRun, error) {
	return orchestrationdb.WorktreeRun{}, nil
}
func (s *runOnlyCoordinatorStub) RemoveManagedWorktree(context.Context, string, bool) (orchestrationdb.WorktreeRun, error) {
	return orchestrationdb.WorktreeRun{}, nil
}
func (s *runOnlyCoordinatorStub) GetLongHorizonState(string) string          { return "" }
func (s *runOnlyCoordinatorStub) GetLongHorizonAuditTail(string, int) string { return "" }
func (s *runOnlyCoordinatorStub) RunPlanMode(context.Context, string, string, string) (*agentformula.ExecutionState, error) {
	return nil, nil
}
func (s *runOnlyCoordinatorStub) ResolvePlanApproval(context.Context, string, bool) error {
	return nil
}

func TestRunNonInteractiveDoesNotPrintSyntheticFallback(t *testing.T) {
	progress := false
	app := &App{
		Sessions:         &sessionServiceStub{},
		Messages:         &messageServiceStub{},
		Permissions:      permission.NewPermissionService(t.TempDir(), true, nil),
		AgentCoordinator: &runOnlyCoordinatorStub{},
		config: &config.Config{
			Options: &config.Options{Progress: &progress},
		},
	}

	var output bytes.Buffer
	err := app.RunNonInteractive(t.Context(), &output, "hi", "", "", "", true)
	require.NoError(t, err)
	require.Equal(t, "\n", output.String())
}

type subscriberFixture struct {
	broker   *pubsub.Broker[string]
	wg       sync.WaitGroup
	outputCh chan tea.Msg
	cancel   context.CancelFunc
}

func newSubscriberFixture(t *testing.T, bufSize int, mode subscriberMode) *subscriberFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	f := &subscriberFixture{
		broker:   pubsub.NewBroker[string](),
		outputCh: make(chan tea.Msg, bufSize),
		cancel:   cancel,
	}
	t.Cleanup(f.broker.Shutdown)

	setupSubscriber(ctx, &f.wg, "test", func(ctx context.Context) <-chan pubsub.Event[string] {
		return f.broker.Subscribe(ctx)
	}, f.outputCh, mode)

	return f
}
