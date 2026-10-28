package service

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	ber "github.com/go-asn1-ber/asn1-ber"
	ldap "github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
)

func TestStockIdempotencyKeyRejectsChangedBusinessCommand(t *testing.T) {
	actor, stock, key, destination, device := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	base := SpareMoveInput{SubmissionID: key, Operation: "TRANSFER", Quantity: 2, Reason: "库房调拨", DestinationSiteID: &destination}
	for _, field := range []string{"operation", "quantity", "reason", "destination", "device", "initial state"} {
		t.Run(field, func(t *testing.T) {
			svc, m, closeDB := opsMock(t)
			defer closeDB()
			in := base
			switch field {
			case "operation":
				in.Operation = "OUT"
			case "quantity":
				in.Quantity++
			case "reason":
				in.Reason = "其他用途"
			case "destination":
				other := uuid.New()
				in.DestinationSiteID = &other
			case "device":
				in.DeviceID = &device
			case "initial state":
				in.receiptState = "FAULTY"
			}
			m.ExpectBegin()
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id", "stock_id", "actor_id", "submission_id", "operation", "quantity", "reason", "destination_site_id", "after_state"}).AddRow(uuid.New(), stock, actor, key, base.Operation, base.Quantity, base.Reason, destination, "TRANSIT"))
			m.ExpectRollback()
			_, err := svc.MoveStock(context.Background(), Actor{UserID: &actor}, stock, in)
			if !errors.Is(err, ErrInvalidResource) {
				t.Fatalf("changed %s silently accepted: %v", field, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStockReplayHandlesDerivedFieldsAndDisposalEvidence(t *testing.T) {
	device, destination := uuid.New(), uuid.New()
	for _, in := range []SpareMoveInput{
		{Operation: "REMOVE", Quantity: 1, Reason: "拆回检测"},
		{Operation: "RECEIVE", Quantity: 2, Reason: "确认到货"},
		{Operation: "RETIRE", Quantity: 1, Reason: "损坏报废", DisposalEvidence: "擦除验证单 001"},
	} {
		prior := model.SpareMovement{Operation: in.Operation, Quantity: in.Quantity, Reason: spareMoveReason(in)}
		if in.Operation == "REMOVE" {
			prior.DeviceID = &device
		}
		if in.Operation == "RECEIVE" {
			prior.DestinationSiteID = &destination
		}
		if !sameSpareMove(prior, in) {
			t.Fatalf("legitimate %s retry rejected", in.Operation)
		}
		if in.Operation == "RETIRE" {
			in.DisposalEvidence = "不同的擦除证明"
			if sameSpareMove(prior, in) {
				t.Fatal("different disposal evidence accepted as original command")
			}
		}
	}
}

// A directory that accepts TCP and then never answers must not outlive the
// caller's request, including after the StartTLS extended-operation response.
func TestLDAPCancellationBoundsSilentBindAndTLSHandshake(t *testing.T) {
	for _, startTLS := range []bool{false, true} {
		name := "bind"
		if startTLS {
			name = "starttls handshake"
		}
		t.Run(name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			serverDone := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					serverDone <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				if startTLS {
					request, err := ber.ReadPacket(conn)
					if err != nil {
						serverDone <- err
						return
					}
					_, err = conn.Write(ldapResultPacket(request.Children[0].Value.(int64), ldap.ApplicationExtendedResponse).Bytes())
					if err != nil {
						serverDone <- err
						return
					}
				}
				_, _ = io.Copy(io.Discard, conn)
				serverDone <- nil
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			started := time.Now()
			err = NewLDAPService(nil).Test(ctx, model.LDAPConfig{URL: "ldap://" + listener.Addr().String(), BindDN: "cn=service", BindPassword: "test-secret", UseTLS: startTLS, SkipTLSVerify: true})
			if err == nil || time.Since(started) > time.Second {
				t.Fatalf("silent LDAP request did not cancel promptly: %v, duration %s", err, time.Since(started))
			}
			if err := <-serverDone; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func ldapResultPacket(id int64, tag ber.Tag) *ber.Packet {
	packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP result")
	packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, id, "message id"))
	result := ber.Encode(ber.ClassApplication, ber.TypeConstructed, tag, nil, "result")
	result.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, 0, "success"))
	result.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "matched DN"))
	result.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "", "diagnostic"))
	packet.AppendChild(result)
	return packet
}

func TestLDAPStartTLSUsesDirectoryHostname(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	hostname := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			hostname <- "accept failed"
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		request, err := ber.ReadPacket(conn)
		if err != nil {
			hostname <- "LDAP request failed"
			return
		}
		if _, err := conn.Write(ldapResultPacket(request.Children[0].Value.(int64), ldap.ApplicationExtendedResponse).Bytes()); err != nil {
			hostname <- "LDAP response failed"
			return
		}
		server := tls.Server(conn, &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			hostname <- hello.ServerName
			return nil, errors.New("test stops after reading TLS ClientHello")
		}})
		_ = server.Handshake()
	}()
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = NewLDAPService(nil).Test(ctx, model.LDAPConfig{URL: "ldap://localhost:" + port, UseTLS: true})
	select {
	case got := <-hostname:
		if got != "localhost" {
			t.Fatalf("STARTTLS did not verify configured hostname: %q", got)
		}
	case <-ctx.Done():
		t.Fatal("server did not receive TLS ClientHello")
	}
}

func TestLDAPGroupSearchAcceptsThreeMemberships(t *testing.T) {
	client, server := net.Pipe()
	conn := ldap.NewConn(client, false)
	conn.SetTimeout(time.Second)
	conn.Start()
	defer conn.Close()
	defer server.Close()
	serverDone := make(chan error, 1)
	go func() {
		request, err := ber.ReadPacket(server)
		if err != nil {
			serverDone <- err
			return
		}
		id := request.Children[0].Value.(int64)
		// Simulate a directory enforcing the client's requested size limit.
		limit := request.Children[1].Children[3].Value.(int64)
		groups := []string{"users", "operators", "asset-admins"}
		for _, group := range groups {
			packet := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP entry")
			packet.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, id, "message id"))
			entry := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ldap.ApplicationSearchResultEntry, nil, "entry")
			entry.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "cn="+group+",dc=example", "DN"))
			attributes := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attributes")
			attribute := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "attribute")
			attribute.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, "cn", "name"))
			values := ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSet, nil, "values")
			values.AppendChild(ber.NewString(ber.ClassUniversal, ber.TypePrimitive, ber.TagOctetString, group, "value"))
			attribute.AppendChild(values)
			attributes.AppendChild(attribute)
			entry.AppendChild(attributes)
			packet.AppendChild(entry)
			if _, err := server.Write(packet.Bytes()); err != nil {
				serverDone <- err
				return
			}
		}
		result := ldapResultPacket(id, ldap.ApplicationSearchResultDone)
		if limit > 0 && limit < int64(len(groups)) {
			result.Children[1].Children[0] = ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagEnumerated, ldap.LDAPResultSizeLimitExceeded, "size limit")
			// Rebuild the application packet so its encoded data contains the error.
			failed := ber.Encode(ber.ClassApplication, ber.TypeConstructed, ldap.ApplicationSearchResultDone, nil, "result")
			for _, field := range result.Children[1].Children {
				failed.AppendChild(field)
			}
			result = ber.Encode(ber.ClassUniversal, ber.TypeConstructed, ber.TagSequence, nil, "LDAP result")
			result.AppendChild(ber.NewInteger(ber.ClassUniversal, ber.TypePrimitive, ber.TagInteger, id, "id"))
			result.AppendChild(failed)
		}
		_, err = server.Write(result.Bytes())
		serverDone <- err
	}()
	groups, err := searchUserGroups(conn, model.LDAPConfig{BaseDN: "dc=example", GroupFilter: "(member={{userDN}})", GroupAttribute: "cn"}, "uid=alice,dc=example", "alice")
	if err != nil || len(groups) != 3 || groups[2] != "asset-admins" {
		t.Fatalf("multi-group account cannot authenticate: %#v, %v", groups, err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestChangedWarrantyConditionRequiresNewAcknowledgement(t *testing.T) {
	for _, changed := range []bool{false, true} {
		name := "unchanged condition retains acknowledgement"
		if changed {
			name = "changed expiry resets acknowledgement"
		}
		t.Run(name, func(t *testing.T) {
			ops, m, closeDB := opsMock(t)
			defer closeDB()
			deviceID, reminderID, actorID := uuid.New(), uuid.New(), uuid.New()
			end := dateOnly(time.Now().In(chinaZone)).AddDate(0, 0, -1)
			detail := "SRV-1 / SN SN-1 / " + end.Format("2006-01-02")
			oldDetail := detail
			if changed {
				oldDetail = "SRV-1 / SN SN-1 / " + end.AddDate(0, 0, -1).Format("2006-01-02")
			}
			m.ExpectBegin()
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .*devices`).WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "serial_number", "warranty_expires_at"}).AddRow(deviceID, "SRV-1", "服务器", "SN-1", end))
			m.ExpectQuery(`SELECT .*rack_device_positions`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*asset_coverages`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*asset_reminders.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "active", "title", "detail", "severity", "acknowledged_by", "acknowledgement"}).AddRow(reminderID, 2, true, "服务器 · 已过保", oldDetail, "DANGER", actorID, "旧告警已核查"))
			m.ExpectExec(`UPDATE .*asset_reminders`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .*spare_minimums`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*notification_settings`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*racks`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectExec(`UPDATE .*asset_reminders`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			var saved *model.AssetReminder
			ops.DB.Callback().Update().Before("gorm:update").Register("capture_refreshed_warranty", func(tx *gorm.DB) {
				if reminder, ok := tx.Statement.Dest.(*model.AssetReminder); ok {
					copy := *reminder
					saved = &copy
				}
			})
			if err := ops.RefreshReminders(context.Background()); err != nil {
				t.Fatal(err)
			}
			if saved == nil || saved.Detail != detail || (saved.AcknowledgedBy == nil) != changed {
				t.Fatalf("incorrect acknowledgement after condition refresh: %+v", saved)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
