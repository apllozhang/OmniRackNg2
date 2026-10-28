package service

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"testing"
)

func TestNormalizeScanBaseURL(t *testing.T) {
	for _, tt := range []struct{ in, want string }{{"", ""}, {" https://Assets.example.com/ ", "https://assets.example.com"}, {"http://203.0.113.10:15173", "http://203.0.113.10:15173"}, {"https://[2001:db8::1]:8443/", "https://[2001:db8::1]:8443"}} {
		got, err := NormalizeScanBaseURL(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("%q: %q %v", tt.in, got, err)
		}
	}
	for _, in := range []string{"example.com", "javascript:alert(1)", "ftp://example.com", "https://u:p@example.com", "https://example.com/path", "https://example.com?a=1", "https://example.com?", "https://example.com#", "https://example.com:0", "https://example.com:65536", "https://exam ple.com", "https://bad..host", "https://example.com/a/.."} {
		if _, err := NormalizeScanBaseURL(in); err == nil {
			t.Errorf("accepted %q", in)
		}
	}
}
func TestReadScanAccessDefaultAndPersisted(t *testing.T) {
	for _, exists := range []bool{false, true} {
		s, m, closeDB := opsMock(t)
		rows := sqlmock.NewRows([]string{"id", "version", "base_url"})
		if exists {
			rows.AddRow(1, 2, "https://assets.example.com")
		}
		m.ExpectQuery(`SELECT .*scan_access_settings`).WillReturnRows(rows)
		got, err := s.ScanAccess(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if exists && (got.Version != 2 || got.BaseURL != "https://assets.example.com") {
			t.Fatal(got)
		}
		if !exists && (got.Version != 0 || got.BaseURL != "") {
			t.Fatal(got)
		}
		if err = m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		closeDB()
	}
}
func TestSaveScanAccessVersionAuditAndRollback(t *testing.T) {
	for _, mode := range []string{"save", "stale", "audit-failure", "clear"} {
		t.Run(mode, func(t *testing.T) {
			s, m, closeDB := opsMock(t)
			defer closeDB()
			m.ExpectBegin()
			m.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(338743, 3).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .*scan_access_settings`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "base_url"}).AddRow(1, 2, "https://old.example.com"))
			version := uint(2)
			if mode == "stale" {
				version = 1
				m.ExpectRollback()
			} else {
				m.ExpectExec(`UPDATE "scan_access_settings"`).WillReturnResult(sqlmock.NewResult(0, 1))
				if mode == "audit-failure" {
					m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnError(errors.New("audit unavailable"))
					m.ExpectRollback()
				} else {
					m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
					m.ExpectCommit()
				}
			}
			address := "https://new.example.com/"
			if mode == "clear" {
				address = ""
			}
			got, err := s.SaveScanAccess(context.Background(), Actor{}, model.ScanAccessSettings{Version: version, BaseURL: address})
			if mode == "stale" && !errors.Is(err, ErrResourceVersion) {
				t.Fatal(err)
			}
			if mode == "audit-failure" && err == nil {
				t.Fatal("audit error ignored")
			}
			if mode == "save" && (err != nil || got.Version != 3 || got.BaseURL != "https://new.example.com") {
				t.Fatal(got, err)
			}
			if mode == "clear" && (err != nil || got.BaseURL != "") {
				t.Fatal(got, err)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCreateInitialScanAccess(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	m.ExpectBegin()
	m.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*scan_access_settings`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "base_url"}))
	m.ExpectQuery(`INSERT INTO "scan_access_settings"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	got, err := s.SaveScanAccess(context.Background(), Actor{}, model.ScanAccessSettings{BaseURL: "https://assets.example.com"})
	if err != nil || got.Version != 1 || got.ID != 1 {
		t.Fatal(got, err)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
