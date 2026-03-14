.PHONY: single-up single-down single-logs \
        multi-up multi-down multi-logs \
        clean

## single-up: start standalone single-DB postgres container
single-up:
	docker compose -f docker-compose.single.yml up -d

## single-down: stop single-DB container (keeps volume)
single-down:
	docker compose -f docker-compose.single.yml down

## single-logs: tail logs for single-DB container
single-logs:
	docker compose -f docker-compose.single.yml logs -f

## multi-up: start standalone multi-DB postgres container
multi-up:
	docker compose -f docker-compose.multi.yml up -d

## multi-down: stop multi-DB container (keeps volume)
multi-down:
	docker compose -f docker-compose.multi.yml down

## multi-logs: tail logs for multi-DB container
multi-logs:
	docker compose -f docker-compose.multi.yml logs -f

## clean: remove all DB containers AND volumes
clean:
	docker compose -f docker-compose.single.yml down -v
	docker compose -f docker-compose.multi.yml down -v
