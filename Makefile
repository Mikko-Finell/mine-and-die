.PHONY: run sync stop cleanbranches test

run:
	SYNC=0 FOLLOW=0 ./dev-follow.sh

sync:
	SYNC=1 FOLLOW=1 ./dev-follow.sh

stop:
	-kill $$(cat server/bin/server.pid 2>/dev/null) 2>/dev/null || true
	@rm -f server/bin/server.pid

cleanbranches:
	@git branch | grep -v "main" | xargs git branch -D 2>/dev/null || true
	@git fetch --prune
	@echo "✅ Cleaned all local branches except 'main' and pruned remotes."

test:
	npm test
	(cd server && go test ./...)
