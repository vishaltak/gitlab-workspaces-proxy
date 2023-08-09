package sshproxy

import (
	"context"
	"io"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

func (p *sshProxy) copyRequests(reqChannel <-chan *ssh.Request, conn ssh.Conn) {
	for req := range reqChannel {
		result, payload, err := conn.SendRequest(req.Type, req.WantReply, req.Payload)
		if err != nil {
			continue
		}
		err = req.Reply(result, payload)
		if err != nil {
			continue
		}
	}
}

func (p *sshProxy) copyData(ctx context.Context, target ssh.Conn, source <-chan ssh.NewChannel) { //nolint:cyclop
	for srcCh := range source {
		ctx, cancel := context.WithCancel(ctx)

		go func(s ssh.NewChannel) {
			targetChannel, targetRequestChannel, err := target.OpenChannel(s.ChannelType(), s.ExtraData())
			if err != nil {
				p.log.Error("Error copying data", zap.Error(err))
				return
			}

			sourceChannel, sourceRequestChannel, err := s.Accept()
			if err != nil {
				p.log.Error("Error copying data", zap.Error(err))
				return
			}
			defer func() {
				err = sourceChannel.Close()
				if err != nil {
					p.log.Error("Error closing source channel", zap.Error(err))
				}
			}()

			go func() {
				_, err = io.Copy(targetChannel, sourceChannel)
				if err != nil {
					p.log.Error("Error copying data to target channel ", zap.Error(err))
				}
				err = targetChannel.CloseWrite()
				if err != nil {
					p.log.Error("Unable to close target channel ", zap.Error(err))
				}
			}()

			go func() {
				_, err = io.Copy(sourceChannel, targetChannel)
				if err != nil {
					p.log.Error("Error copying data to source channel ", zap.Error(err))
				}
				err = sourceChannel.CloseWrite()
				if err != nil {
					p.log.Error("Unable to close source channel ", zap.Error(err))
				}
			}()

			go func() {
				defer cancel()

				for {
					req, ok := <-targetRequestChannel

					if !ok {
						err = sourceChannel.Close()
						p.log.Error("Unable to close source channel ", zap.Error(err))
						return
					}

					b, err := sourceChannel.SendRequest(req.Type, req.WantReply, req.Payload)
					if err != nil {
						p.log.Error("Error copying data", zap.Error(err))
						return
					}
					err = req.Reply(b, nil)
					if err != nil {
						return
					}
				}
			}()

			go func() {
				defer cancel()

				for {
					req, ok := <-sourceRequestChannel

					if !ok {
						err = targetChannel.Close()
						p.log.Error("Unable to close target channel ", zap.Error(err))
						return
					}

					b, err := targetChannel.SendRequest(req.Type, req.WantReply, req.Payload)
					if err != nil {
						p.log.Error("Error copying data", zap.Error(err))
						return
					}
					err = req.Reply(b, nil)
					if err != nil {
						return
					}
				}
			}()

			<-ctx.Done()
		}(srcCh)
	}
}
