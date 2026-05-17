package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/medigt/medigt/server/internal/cache"
	"github.com/medigt/medigt/server/internal/config"
	"github.com/medigt/medigt/server/internal/handler"
	"github.com/medigt/medigt/server/internal/integration/enabiz"
	"github.com/medigt/medigt/server/internal/integration/hl7"
	"github.com/medigt/medigt/server/internal/integration/erecete"
	"github.com/medigt/medigt/server/internal/integration/its"
	"github.com/medigt/medigt/server/internal/integration/medula"
	"github.com/medigt/medigt/server/internal/integration/mernis"
	"github.com/medigt/medigt/server/internal/integration/pacs"
	"github.com/medigt/medigt/server/internal/integration/turkkep"
	"github.com/medigt/medigt/server/internal/logger"
	"github.com/medigt/medigt/server/internal/realtime"
	"github.com/medigt/medigt/server/internal/service"
	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	log := logger.New()
	log.Info("medigt server starting", "version", version, "commit", commit, "date", date)

	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Error("database ping failed", "err", err)
		os.Exit(1)
	}
	log.Info("database connected")

	cacheClient, err := cache.NewRedis(cfg.RedisURL)
	if err != nil {
		log.Error("cache init failed", "err", err)
		os.Exit(1)
	}
	if cfg.RedisURL == "" {
		log.Warn("REDIS_URL not set — using in-process cache (single-node only)")
	}

	hub := realtime.NewHub(log)
	go hub.Run(ctx)

	// Repos
	users := repo.NewUserRepo(pool)
	sessions := repo.NewSessionRepo(pool)
	orgs := repo.NewOrganizationRepo(pool)
	branches := repo.NewBranchRepo(pool)
	memberships := repo.NewMembershipRepo(pool)
	specializations := repo.NewSpecializationRepo(pool)
	staff := repo.NewStaffRepo(pool)
	doctors := repo.NewDoctorRepo(pool)
	institutions := repo.NewInstitutionRepo(pool)
	services := repo.NewServiceCatalogRepo(pool)
	servicePrices := repo.NewServicePriceRepo(pool)
	icd10 := repo.NewIcd10Repo(pool)
	patients := repo.NewPatientRepo(pool)
	patientSvc := service.NewPatientService(patients)
	appointments := repo.NewAppointmentRepo(pool)
	visits := repo.NewVisitRepo(pool)
	visitSvc := service.NewVisitService(pool, visits)
	diagnoses := repo.NewDiagnosisRepo(pool)
	prescriptions := repo.NewPrescriptionRepo(pool)
	vitals := repo.NewVitalSignsRepo(pool)
	lab := repo.NewLabRepo(pool)
	radiology := repo.NewRadiologyRepo(pool)
	wards := repo.NewWardRepo(pool)
	beds := repo.NewBedRepo(pool)
	admissions := repo.NewAdmissionRepo(pool)
	admissionSvc := service.NewAdmissionService(pool)
	operatingRooms := repo.NewOperatingRoomRepo(pool)
	surgeries := repo.NewSurgeryRepo(pool)
	dialysisMachines := repo.NewDialysisMachineRepo(pool)
	dialysisSessions := repo.NewDialysisSessionRepo(pool)
	medications := repo.NewMedicationRepo(pool)
	warehouses := repo.NewWarehouseRepo(pool)
	stock := repo.NewStockRepo(pool)
	movements := repo.NewMovementRepo(pool)
	stockSvc := service.NewStockService(pool)
	eczane := repo.NewEczaneRepo(pool)
	cashRegisters := repo.NewCashRegisterRepo(pool)
	cashMovements := repo.NewCashMovementRepo(pool)
	cashSvc := service.NewCashService(pool)
	invoices := repo.NewInvoiceRepo(pool)
	payments := repo.NewPaymentRepo(pool)
	invoiceSvc := service.NewInvoiceService(pool)
	hakedis := repo.NewHakedisRepo(pool)
	mernisRepo := repo.NewMernisRepo(pool)
	medulaRepo := repo.NewMedulaRepo(pool)
	cariRepo := repo.NewCariRepo(pool)
	refundRepo := repo.NewRefundRepo(pool)
	installmentRepo := repo.NewInstallmentPlanRepo(pool)
	financeExt := service.NewFinanceExtensionsService(pool)

	// Integration clients — mock today, real impl swaps in when SGK + NVI
	// production creds are wired through cfg.
	mernisClient := mernis.NewMockClient()
	medulaClient := medula.NewMockClient()
	ereceteClient := erecete.NewFromConfig(erecete.HTTPConfig{
		BaseURL:      cfg.EreceteBaseURL,
		ClientID:     cfg.EreceteClientID,
		ClientSecret: cfg.EreceteClientSecret,
		Logger:       log,
	})
	itsClient := its.NewFromConfig(its.HTTPConfig{
		BaseURL:      cfg.ITSBaseURL,
		ClientID:     cfg.ITSClientID,
		ClientSecret: cfg.ITSClientSecret,
		Logger:       log,
	})
	turkkepClient := turkkep.NewMockClient()
	signatureRepo := repo.NewSignatureRepo(pool)
	signatureSvc := service.NewSignatureService(pool, signatureRepo, turkkepClient)
	pacsClient := pacs.NewMockClient()
	imageRefRepo := repo.NewImageReferenceRepo(pool)
	labHL7Svc := service.NewLabHL7Service(pool, lab)
	auditRepo := repo.NewAuditRepo(pool)
	marRepo := repo.NewMARRepo(pool)
	enabizRepo := repo.NewEnabizRepo(pool)
	enabizClient := enabiz.NewMockClient()
	hl7OutboundRepo := repo.NewHL7OutboundRepo(pool)
	adtSvc := service.NewADTService(pool, hl7OutboundRepo, log)
	// Dispatcher: HL7_ADT_PEER_ADDRESS boşsa MockDispatcher (log + AA ack);
	// doluysa MLLPDispatcher → TCP MLLP üzerinden gerçek PACS/LIS/HIE peer.
	hl7Dispatcher := hl7.NewDispatcher(cfg.HL7ADTPeerAddress, cfg.HL7ADTPeerTimeout, log)
	adtWorker := hl7.NewADTOutboxWorker(pool, hl7OutboundRepo, hl7Dispatcher, log)
	go adtWorker.Run(ctx)

	// Medula outbox worker — single goroutine; safe to scale horizontally
	// because ClaimNext uses FOR UPDATE SKIP LOCKED.
	medulaWorker := medula.NewOutboxWorker(pool, medulaRepo, medulaClient, ereceteClient, itsClient, log)
	go medulaWorker.Run(ctx)

	// e-Nabız outbox worker — independent goroutine so Bakanlık outages
	// don't poison SGK flow.
	enabizWorker := enabiz.NewOutboxWorker(pool, enabizRepo, enabizClient, log)
	go enabizWorker.Run(ctx)

	// Emailer chain: SMTP first (Mailhog/Resend SMTP relay), stdout fallback.
	emailer := service.NewChainEmailer(log,
		service.NewSMTPEmailer(log, cfg.ResendFromEmail),
		service.NewStdoutEmailer(log),
	)

	// Services
	codes := service.NewCodeService(cacheClient, !cfg.IsProduction())
	authSvc := service.NewAuthService(service.AuthDeps{
		Users:        users,
		Sessions:     sessions,
		Codes:        codes,
		Emailer:      emailer,
		JWTSecret:    cfg.JWTSecret,
		AllowSignup:  cfg.AllowSignup,
		EmailDomains: cfg.AllowedEmailDomains,
		EmailList:    cfg.AllowedEmails,
	})
	tenantSvc := service.NewTenantService(service.TenantDeps{
		Orgs:        orgs,
		Branches:    branches,
		Memberships: memberships,
	})

	h := handler.New(handler.Deps{
		Log:             log,
		Pool:            pool,
		Hub:             hub,
		Cfg:             cfg,
		Auth:            authSvc,
		Tenant:          tenantSvc,
		Users:           users,
		Sessions:        sessions,
		Orgs:            orgs,
		Branches:        branches,
		Memberships:     memberships,
		Specializations: specializations,
		Staff:           staff,
		Doctors:         doctors,
		Institutions:    institutions,
		Services:        services,
		ServicePrices:   servicePrices,
		Icd10:           icd10,
		Patients:        patients,
		PatientSvc:      patientSvc,
		Appointments:    appointments,
		Visits:          visits,
		VisitSvc:        visitSvc,
		Diagnoses:       diagnoses,
		Prescriptions:   prescriptions,
		Vitals:          vitals,
		Lab:             lab,
		Radiology:       radiology,
		Wards:           wards,
		Beds:            beds,
		Admissions:      admissions,
		AdmissionSvc:    admissionSvc,
		OperatingRooms:  operatingRooms,
		Surgeries:       surgeries,
		DialysisMachines: dialysisMachines,
		DialysisSessions: dialysisSessions,
		Medications:     medications,
		Warehouses:      warehouses,
		Stock:           stock,
		Movements:       movements,
		StockSvc:        stockSvc,
		Eczane:          eczane,
		CashRegisters:   cashRegisters,
		CashMovements:   cashMovements,
		CashSvc:         cashSvc,
		Invoices:        invoices,
		Payments:        payments,
		InvoiceSvc:      invoiceSvc,
		Hakedis:         hakedis,
		Mernis:          mernisRepo,
		MernisSvc:       mernisClient,
		Medula:          medulaRepo,
		MedulaClient:    medulaClient,
		Cari:            cariRepo,
		Refunds:         refundRepo,
		Installments:    installmentRepo,
		FinanceExt:      financeExt,
		SignatureRepo:   signatureRepo,
		Signatures:      signatureSvc,
		PACSClient:      pacsClient,
		ImageRefs:       imageRefRepo,
		LabHL7:          labHL7Svc,
		Audit:           auditRepo,
		MAR:             marRepo,
		Enabiz:          enabizRepo,
		HL7Outbound:     hl7OutboundRepo,
		ADT:             adtSvc,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           h.Router(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("HTTP listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		log.Error("HTTP server failed", "err", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown failed", "err", err)
	}
	log.Info("medigt server stopped")
}
