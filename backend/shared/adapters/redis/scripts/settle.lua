-- Settles a delivered entry, but only if ARGV[2] still owns it.
-- KEYS[1] ready stream, KEYS[2] delayed sorted set, KEYS[3] dead-letter stream
-- ARGV[1] group, ARGV[2] consumer, ARGV[3] entry ID,
-- ARGV[4] action: ack | retry | dead, ARGV[5] payload (retry/dead),
-- ARGV[6] ready-at score (retry), ARGV[7] dead-letter stream max length
-- Returns 1 if settled, 0 if the entry is no longer pending to this consumer.
local p = redis.call('XPENDING', KEYS[1], ARGV[1], ARGV[3], ARGV[3], 1)
if #p == 0 or p[1][2] ~= ARGV[2] then
  return 0
end
redis.call('XACK', KEYS[1], ARGV[1], ARGV[3])
redis.call('XDEL', KEYS[1], ARGV[3])
if ARGV[4] == 'retry' then
  redis.call('ZADD', KEYS[2], ARGV[6], ARGV[5])
elseif ARGV[4] == 'dead' then
  redis.call('XADD', KEYS[3], 'MAXLEN', '~', ARGV[7], '*', 'm', ARGV[5])
end
return 1
