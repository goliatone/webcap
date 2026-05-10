package webcap

import (
	"context"

	command "github.com/goliatone/go-command"
	"github.com/goliatone/webcap/pkg/agents/skills"
)

type SkillInstallMessage struct {
	Request skills.InstallRequest
}

func (SkillInstallMessage) Type() string { return "webcap::skill_install" }

type SkillInstallHandler struct{}

func NewSkillInstallHandler() *SkillInstallHandler {
	return &SkillInstallHandler{}
}

func (h *SkillInstallHandler) Execute(ctx context.Context, msg SkillInstallMessage) error {
	_, err := h.Handle(ctx, msg)
	return err
}

func (h *SkillInstallHandler) Handle(ctx context.Context, msg SkillInstallMessage) (skills.InstallResult, error) {
	return skills.Install(ctx, msg.Request)
}

var _ command.Commander[SkillInstallMessage] = (*SkillInstallHandler)(nil)
