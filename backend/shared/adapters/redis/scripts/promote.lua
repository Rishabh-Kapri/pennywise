-- Moves delayed messages that are due onto the ready stream.
-- KEYS[1] ready stream, KEYS[2] delayed sorted set
-- ARGV[1] now (unix ms), ARGV[2] max messages to move
-- Returns the number moved.
local due = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[2]))
for _, m in ipairs(due) do
  redis.call('XADD', KEYS[1], '*', 'm', m)
  redis.call('ZREM', KEYS[2], m)
end
return #due
