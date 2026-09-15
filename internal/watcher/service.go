package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	"claim-watcher/internal/config"
	dbrepo "claim-watcher/internal/db"
	"claim-watcher/internal/mailer"
)

type Service struct {
	cfg    config.Config
	repo   *dbrepo.Repository
	mailer *mailer.Mailer
}

func New(cfg config.Config, repo *dbrepo.Repository, m *mailer.Mailer) *Service {
	return &Service{cfg: cfg, repo: repo, mailer: m}
}

func (s *Service) Run(ctx context.Context) error {
	runID, err := s.repo.StartRun(ctx)
	if err != nil {
		return err
	}

	log.Printf("Claim Watcher run started: %s", runID)

	stats := dbrepo.RunStats{
		Status: "RUNNING",
	}

	fail := func(runErr error) error {
		msg := runErr.Error()
		stats.Status = "FAILED"
		stats.ErrorMessage = &msg
		if finishErr := s.repo.FinishRun(context.Background(), runID, stats); finishErr != nil {
			log.Printf("ERROR: could not mark watcher run failed: %v", finishErr)
		}
		return runErr
	}

	activeCount, err := s.repo.CountActiveClaims(ctx)
	if err != nil {
		return fail(err)
	}
	stats.ClaimsChecked = activeCount

	claims, err := s.repo.FindDeadlineClaims(ctx, s.cfg.WarningDays)
	if err != nil {
		return fail(err)
	}

	if len(claims) == 0 {
		stats.Status = "NO_ACTION"
		if err := s.repo.FinishRun(ctx, runID, stats); err != nil {
			return err
		}
		log.Printf("No claims are overdue or due within %d days. No email sent.", s.cfg.WarningDays)
		return nil
	}

	digest := BuildDigest(claims, s.cfg.ClaimsBaseURL, s.cfg.WarningDays, time.Now().In(s.cfg.Timezone))
	stats.OverdueClaims = digest.Overdue
	stats.UpcomingClaims = digest.Upcoming

	log.Printf("Matched claims: %d overdue, %d current/upcoming", digest.Overdue, digest.Upcoming)

	if err := s.mailer.Send(digest.Subject, digest.PlainBody, digest.HTMLBody); err != nil {
		return fail(fmt.Errorf("send digest: %w", err))
	}
	stats.EmailSent = true

	recorded, err := s.repo.RecordNotifications(ctx, runID, s.cfg.SMTPTo, s.cfg.WarningDays, claims)
	if err != nil {
		return fail(err)
	}
	stats.NotificationsRecorded = recorded

	stats.Status = "SUCCESS"
	if err := s.repo.FinishRun(ctx, runID, stats); err != nil {
		return err
	}

	log.Printf("Digest sent successfully to %s; %d notification audit rows recorded.", s.cfg.SMTPTo, recorded)
	return nil
}
