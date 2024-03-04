package sshproxy

import (
	"context"
	"io"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"golang.org/x/crypto/ssh"
)

func (p *SSHProxy) copyRequests(ctx context.Context, reqChannel <-chan *ssh.Request, conn ssh.Conn) {
	workspaceName := ctx.Value(workspaceNameCtxValueKey).(string)
	for req := range reqChannel {
		p.log.Debug("attempting to send request to connection", logz.WorkspaceName(workspaceName))
		result, payload, err := conn.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			p.log.Error("failed to send request to connection", logz.Error(err), logz.WorkspaceName(workspaceName))
			continue
		}

		p.log.Debug("attempting to send a response to request", logz.WorkspaceName(workspaceName))
		err = req.Reply(result, payload)
		if err != nil {
			p.log.Error("failed to send a response to request", logz.Error(err), logz.WorkspaceName(workspaceName))
			continue
		}
	}
}

// nolint:cyclop
func (p *SSHProxy) copyData(ctx context.Context, target ssh.Conn, source <-chan ssh.NewChannel) {
	workspaceName := ctx.Value(workspaceNameCtxValueKey).(string)
	for srcCh := range source {
		copyDataCtx, copyDataCancel := context.WithCancel(ctx)

		go func(s ssh.NewChannel) {
			p.log.Debug("attempting to open channel for target", logz.WorkspaceName(workspaceName))
			targetChannel, targetRequestChannel, openChannelErr := target.OpenChannel(s.ChannelType(), s.ExtraData())
			if openChannelErr != nil {
				p.log.Error("failed to open channel for target", logz.Error(openChannelErr), logz.WorkspaceName(workspaceName))
				return
			}

			p.log.Debug("attempting to accept channel creation request", logz.WorkspaceName(workspaceName))
			sourceChannel, sourceRequestChannel, err := s.Accept()
			if err != nil {
				p.log.Error("failed to accept channel creation request", logz.Error(err), logz.WorkspaceName(workspaceName))
				return
			}
			defer func() {
				p.log.Debug("attempting to close source channel in defer", logz.WorkspaceName(workspaceName))
				closeErr := sourceChannel.Close()
				if closeErr != nil {
					p.log.Error("failed to close source channel in defer", logz.Error(err), logz.WorkspaceName(workspaceName))
				}

				p.log.Debug("attempting to close target channel", logz.WorkspaceName(workspaceName))
				closeErr = targetChannel.Close()
				if closeErr != nil {
					p.log.Error("failed to close target channel in defer", logz.Error(closeErr), logz.WorkspaceName(workspaceName))
				}
			}()

			go func() {
				p.log.Debug("attempting to copy data to target channel from source channel", logz.WorkspaceName(workspaceName))
				_, copyErr := io.Copy(targetChannel, sourceChannel)
				if copyErr != nil {
					p.log.Error("failed to copy data to target channel from source channel", logz.Error(copyErr), logz.WorkspaceName(workspaceName))
				}

				p.log.Debug("attempting to close write for target channel", logz.WorkspaceName(workspaceName))
				closeErr := targetChannel.CloseWrite()
				if closeErr != nil {
					p.log.Error("failed to close write for target channel", logz.Error(closeErr), logz.WorkspaceName(workspaceName))
				}
			}()

			go func() {
				p.log.Debug("attempting to copy data to source channel from target channel", logz.WorkspaceName(workspaceName))
				_, copyErr := io.Copy(sourceChannel, targetChannel)
				if copyErr != nil {
					p.log.Error("failed to copy data to source channel from target channel", logz.Error(copyErr), logz.WorkspaceName(workspaceName))
				}

				p.log.Debug("attempting to close write for source channel", logz.WorkspaceName(workspaceName))
				closeErr := sourceChannel.CloseWrite()
				if closeErr != nil {
					p.log.Error("failed to close write for source channel", logz.Error(closeErr), logz.WorkspaceName(workspaceName))
				}
			}()

			go func() {
				defer copyDataCancel()

				for {
					req, ok := <-targetRequestChannel

					if !ok {
						p.log.Debug("attempting to close source channel", logz.WorkspaceName(workspaceName))
						closeErr := sourceChannel.Close()
						if closeErr != nil {
							p.log.Error("failed to close source channel", logz.Error(closeErr), logz.WorkspaceName(workspaceName))
						}
						return
					}

					p.log.Debug("attempting to send request to source channel", logz.WorkspaceName(workspaceName))
					b, sendErr := sourceChannel.SendRequest(req.Type, req.WantReply, req.Payload)
					if sendErr != nil {
						p.log.Error("failed to send request to source channel", logz.Error(sendErr), logz.WorkspaceName(workspaceName))
						return
					}

					p.log.Debug("attempting to reply with info received from source channel", logz.WorkspaceName(workspaceName))
					replyErr := req.Reply(b, nil)
					if replyErr != nil {
						p.log.Error("failed to reply with info received from source channel", logz.Error(replyErr), logz.WorkspaceName(workspaceName))
						return
					}
				}
			}()

			go func() {
				defer copyDataCancel()

				for {
					req, ok := <-sourceRequestChannel

					if !ok {
						p.log.Debug("attempting to close target channel", logz.WorkspaceName(workspaceName))
						closeErr := targetChannel.Close()
						if closeErr != nil {
							p.log.Error("failed to close target channel", logz.Error(closeErr), logz.WorkspaceName(workspaceName))
						}
						return
					}

					p.log.Debug("attempting to send request to target channel", logz.WorkspaceName(workspaceName))
					b, sendErr := targetChannel.SendRequest(req.Type, req.WantReply, req.Payload)
					if sendErr != nil {
						p.log.Error("failed to send request to target channel", logz.Error(sendErr), logz.WorkspaceName(workspaceName))
						return
					}

					p.log.Debug("attempting to reply with info received from target channel", logz.WorkspaceName(workspaceName))
					replyErr := req.Reply(b, nil)
					if replyErr != nil {
						p.log.Error("failed to reply with info received from target channel", logz.Error(replyErr), logz.WorkspaceName(workspaceName))
						return
					}
				}
			}()

			<-copyDataCtx.Done()
		}(srcCh)
	}
}
