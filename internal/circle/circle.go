// Package circle decides who sees what.
//
// An account's circle is itself plus its friends. A match is visible to an
// account when its circle contains whoever recorded it or either linked
// player. Only the recorder and the linked players may edit or delete it.
// A live session is visible to the circle of whoever started it, and to the
// accounts linked to its players.
//
// Player names are linked to accounts (`matches.player_a_user` /
// `player_b_user`) by display name or username, but only to accounts in the
// recorder's circle, so typing a stranger's name never shows them your match.
package circle

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/Joshtri/rackup-api/internal/db"
)

// Members is a SQL subquery listing the circle of the account in parameter
// `param` (e.g. "$1"): the account itself and its friends.
func Members(param string) string {
	return fmt.Sprintf(
		"(SELECT %[1]s::uuid UNION ALL SELECT friend_id FROM friendships WHERE user_id = %[1]s::uuid)",
		param)
}

// MatchVisible is a SQL condition on a `matches` row (alias `alias`):
// the account in `param` can see it.
func MatchVisible(alias, param string) string {
	c := Members(param)
	return fmt.Sprintf("(%[1]s.user_id IN %[2]s OR %[1]s.player_a_user IN %[2]s OR %[1]s.player_b_user IN %[2]s)",
		alias, c)
}

// MatchEditable is a SQL condition on a `matches` row: the account in
// `param` recorded it or played in it.
func MatchEditable(alias, param string) string {
	return fmt.Sprintf("(%[1]s.user_id = %[2]s::uuid OR %[1]s.player_a_user = %[2]s::uuid OR %[1]s.player_b_user = %[2]s::uuid)",
		alias, param)
}

// SessionVisible is a SQL condition on a `live_sessions` row: the account
// in `param` can see and control it.
func SessionVisible(alias, param string) string {
	return fmt.Sprintf("(%[1]s.created_by IN %[3]s OR %[1]s.state->>'userA' = %[2]s::text OR %[1]s.state->>'userB' = %[2]s::text)",
		alias, param, Members(param))
}

// linkSide finds the account a player name belongs to, among the recorder's
// circle and the account already linked (so a link survives as long as the
// name still fits it, even after an unfriend). Display name wins over username.
func linkSide(name, linked string) string {
	return fmt.Sprintf(`(
		SELECT u.id FROM users u
		WHERE (u.id = m.user_id OR u.id = m.%[2]s
		       OR u.id IN (SELECT friend_id FROM friendships WHERE user_id = m.user_id))
		  AND (lower(u.display_name) = lower(m.%[1]s) OR u.username = lower(m.%[1]s))
		ORDER BY (lower(u.display_name) = lower(m.%[1]s)) DESC, (u.id = m.%[2]s) DESC, u.username
		LIMIT 1)`, name, linked)
}

var relinkSQL = `UPDATE matches m SET player_a_user = ` + linkSide("player_a", "player_a_user") +
	`, player_b_user = ` + linkSide("player_b", "player_b_user")

// LinkMatches (re)links the players of the given matches to accounts.
// Run it after inserting or editing matches.
func LinkMatches(ctx context.Context, q db.DBTX, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := q.Exec(ctx, relinkSQL+" WHERE m.id = ANY($1)", ids)
	return err
}

// LinkRecordedBy (re)links every match recorded by the given accounts.
// Run it when they become friends, so names typed before now link up.
func LinkRecordedBy(ctx context.Context, q db.DBTX, users ...uuid.UUID) error {
	_, err := q.Exec(ctx, relinkSQL+" WHERE m.user_id = ANY($1)", users)
	return err
}
