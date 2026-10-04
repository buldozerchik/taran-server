package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/buldozerchik/taran-server/internal/clientid"
	"github.com/buldozerchik/taran-server/internal/config"
	"github.com/buldozerchik/taran-server/internal/logx"
	"github.com/buldozerchik/taran-server/internal/provider/vk"
	"github.com/buldozerchik/taran-server/internal/proxy/udprelay"
	"github.com/buldozerchik/taran-server/internal/session"
	"github.com/buldozerchik/taran-server/internal/shutdown"
	"github.com/buldozerchik/taran-server/internal/sub"
	"github.com/buldozerchik/taran-server/internal/tunnel"
	"github.com/buldozerchik/taran-server/internal/wire/rtpopus"
)

// version is populated at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	args := os.Args[1:]

	// Резолв подписки до парсинга даёт обязательный peer для валидации.
	if subURL := config.PeekSubURL(args); subURL != "" {
		sub.SetLogger(logx.New(false))
		s, ferr := sub.Fetch(context.Background(), subURL)
		if ferr != nil {
			log.Fatalf("failed to fetch subscription: %v", ferr)
		}
		if len(s.Nodes) == 0 || s.Nodes[0].URI == nil {
			log.Fatalf("no nodes found in subscription")
		}
		args = append(args, s.Nodes[0].URI.String())
	}

	cfg, err := config.ParseClient(args, os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		log.Fatalf("%v", err)
	}

	if cfg.Obf.GenKey {
		key, gerr := rtpopus.GenKeyHex()
		if gerr != nil {
			log.Fatalf("gen-obf-key: %v", gerr)
		}
		fmt.Println(key)
		return
	}

	logger := logx.New(cfg.Log.Debug)
	logger.Infof("Taran client version=%s", version)

	idPaths := clientid.DefaultPaths()
	id, persisted, err := clientid.Resolve(cfg.ClientID, idPaths)
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}
	if !persisted {
		logger.Warnf("client ID не сохранён ни по одному пути (%v) - будет новым при следующем запуске", idPaths)
	}
	cfg.ClientID = id
	logger.Infof("Client ID: %s", cfg.ClientID)

	if cfg.Tunnel.Enabled() {
		logger.Warnf("ссылка содержит конфиг %s: CLI встроенный туннель не поднимает, запустите WireGuard/AmneziaWG отдельно", cfg.Tunnel.Mode)
		cfg.Tunnel.Mode = tunnel.ModeNone
	}

	ctx, stop := shutdown.Watch(context.Background(), logger)
	defer stop()

	sess, err := session.New(cfg, session.Deps{
		Logger: logger,
		Solver: vk.DefaultManualSolver,
	})
	if err != nil {
		logger.Errorf("%v", err)
		os.Exit(1)
	}

	if err := sess.Run(ctx); err != nil {
		if errors.Is(err, udprelay.ErrFatal) {
			logger.Errorf("fatal: %v", err)
		} else {
			logger.Errorf("%v", err)
		}
		os.Exit(1)
	}
}
