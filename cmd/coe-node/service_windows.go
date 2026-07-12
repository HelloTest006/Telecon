//go:build windows

package main

import (
	"log"

	"golang.org/x/sys/windows/svc"
)

type coeService struct {
	run func()
}

func (m *coeService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}
	done := make(chan struct{})
	go func() {
		m.run()
		close(done)
	}()
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case <-done:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				// run() blocks on signal — force exit
				log.Printf("service stop requested")
				return false, 0
			}
		}
	}
}

func runWindowsService(name string, run func()) error {
	isSvc, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isSvc {
		log.Printf("not running as service; use sc.exe create or install script")
		run()
		return nil
	}
	if name == "" {
		name = "COENode"
	}
	return svc.Run("COENode-"+name, &coeService{run: run})
}
