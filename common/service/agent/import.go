package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	agentTypes "github.com/donknap/dpanel/app/agent/types"
)

func (self *Agent) ImportDPanel(ctx context.Context, archive io.Reader) error {
	if archive == nil {
		return errors.New("import archive is required")
	}
	session, err := self.runner.Stream(ctx, "import", "--dpanel")
	if err != nil {
		return fmt.Errorf("start agent import: %w", err)
	}
	defer session.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stopClose()
	if _, err = io.Copy(session, archive); err != nil {
		_ = session.CloseWrite()
		return fmt.Errorf("send agent import archive: %w", err)
	}
	if err = session.CloseWrite(); err != nil {
		return fmt.Errorf("finish agent import archive: %w", err)
	}
	response, err := io.ReadAll(io.LimitReader(session, 1<<20))
	if err != nil {
		return fmt.Errorf("read agent import response: %w", err)
	}
	message := agentTypes.Message[json.RawMessage]{}
	if err = decodeJSON(response, &message); err != nil {
		return fmt.Errorf("decode agent import response: %w", err)
	}
	if message.Code != 200 || message.Error != "" {
		if message.Error != "" {
			return fmt.Errorf("agent import: %s", message.Error)
		}
		return fmt.Errorf("agent import returned code %d", message.Code)
	}
	return nil
}
