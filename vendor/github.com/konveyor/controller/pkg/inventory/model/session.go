package model

import (
	"database/sql"
	liberr "github.com/konveyor/controller/pkg/error"
	_ "github.com/mattn/go-sqlite3"
)

//
// DB session.
// Encapsulates the sql.DB.
type Session struct {
	// ID.
	id int
	// Return the session to the pool.
	returner func()
	// DB connection.
	db *sql.DB
	// DB transaction history.
	tx []*sql.Tx
	// Closed indicator.
	closed bool
}

//
// Return the session to the pool.
// After is has been returned, it MUST no longer be used.
func (s *Session) Return() {
	s.assertReserved()
	s.returner()
	s.returner = nil
}

//
// Begin a transaction.
func (s *Session) Begin() (tx *sql.Tx, err error) {
	s.assertReserved()
	tx, err = s.db.Begin()
	if err != nil {
		err = liberr.Wrap(err)
	}
	s.tx = append(s.tx, tx)

	return
}

//
// Assert reserved.
// Ensure the session has been reserved and not
// yet returned.
func (s *Session) assertReserved() {
	if s.returner == nil {
		err := liberr.New(
			"operation on returned session.")
		panic(err)
	}
}

//
// Reset.
// Ensure all transactions have been ended.
func (s *Session) reset() {
	for _, tx := range s.tx {
		_ = tx.Rollback()
	}

	s.tx = nil
}

//
// Session pool.
type Pool struct {
	// Journal.
	journal *Journal
	// All sessions.
	sessions []*Session
	// Next (free) sessions.
	next struct {
		writer chan *Session
		reader chan *Session
	}
}

//
// Open the pool.
// Create sessions with DB connections.
// For sqlite3:
//   Even with journal=WAL, nWriter must be (1) to
//   prevent SQLITE_LOCKED error.
func (p *Pool) Open(nWriter, nReader int, path string, journal *Journal) (err error) {
	defer func() {
		if err != nil {
			_ = p.Close()
		}
	}()
	p.journal = journal
	total := nWriter + nReader
	p.next.writer = make(chan *Session, nWriter)
	p.next.reader = make(chan *Session, nReader)
	for id := 0; id < total; id++ {
		session := &Session{id: id}
		session.db, err = sql.Open("sqlite3", path)
		if err != nil {
			return
		}
		pragma := []string{
			"PRAGMA foreign_keys = ON",
			"PRAGMA journal_mode = WAL",
		}
		for _, stmt := range pragma {
			_, err = session.db.Exec(stmt)
			if err != nil {
				return
			}
		}
		p.sessions = append(
			p.sessions,
			session)
		if id < nWriter {
			p.next.writer <- session
		} else {
			p.next.reader <- session
		}
	}

	return
}

//
// Close the pool.
// Close DB connections.
func (p *Pool) Close() (err error) {
	for _, session := range p.sessions {
		_ = session.db.Close()
		session.closed = true
	}

	return
}

//
// Get the next writer.
// This may block until available.
func (p *Pool) Writer() *Session {
	return p.nextSession(p.next.writer)
}

//
// Get the next reader.
// This may block until available.
func (p *Pool) Reader() *Session {
	return p.nextSession(p.next.reader)
}

//
// Get the next session.
// This may block until available.
func (p *Pool) nextSession(ch chan *Session) (session *Session) {
	next := <-ch
	session = &Session{
		id: next.id,
		db: next.db,
		returner: func() {
			session.reset()
			ch <- next
		},
	}

	return
}
