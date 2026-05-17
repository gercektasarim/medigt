package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/medigt/medigt/server/pkg/db/repo"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	root := &cobra.Command{
		Use:   "medigt",
		Short: "MediGt CLI — admin and operational tools for the HBYS",
	}

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("medigt %s (%s, %s)\n", version, commit, date)
		},
	})

	root.AddCommand(seedICD10Cmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// seedICD10Cmd bulk-loads ICD-10-TR codes from a TSV file into the system
// catalog (organization_id = NULL, is_system = TRUE). Idempotent — re-runs
// safely update titles/chapters via ON CONFLICT.
//
// File format (UTF-8, tab-separated):
//
//	code<TAB>title_tr<TAB>chapter[<TAB>parent_code]
//
// Lines starting with '#' and the literal 'code\t…' header row are skipped.
func seedICD10Cmd() *cobra.Command {
	var (
		filePath string
		dryRun   bool
	)
	cmd := &cobra.Command{
		Use:   "seed-icd10",
		Short: "Bulk-load ICD-10-TR codes from a TSV file (system catalog)",
		Long: `Loads ICD-10-TR codes into the shared system catalog. By default reads
server/data/icd10-tr-extended.tsv (~362 codes); pass --file for a custom path
(e.g. the full ~14000 row Sağlık Bakanlığı list).

Requires DATABASE_URL.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			dbURL := os.Getenv("DATABASE_URL")
			if dbURL == "" {
				return fmt.Errorf("DATABASE_URL boş")
			}
			path := filePath
			if path == "" {
				path = "server/data/icd10-tr-extended.tsv"
			}
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("dosya açılamadı (%s): %w", path, err)
			}
			defer f.Close()

			codes, err := parseICD10TSV(f)
			if err != nil {
				return fmt.Errorf("dosya ayrıştırılamadı: %w", err)
			}
			fmt.Printf("• %d ICD-10 kodu ayrıştırıldı\n", len(codes))
			if dryRun {
				for i, c := range codes {
					if i >= 5 {
						fmt.Println("  …")
						break
					}
					fmt.Printf("  %s\t%s\n", c.Code, c.TitleTR)
				}
				return nil
			}

			ctx := context.Background()
			pool, err := pgxpool.New(ctx, dbURL)
			if err != nil {
				return fmt.Errorf("DB bağlantısı kurulamadı: %w", err)
			}
			defer pool.Close()

			icd := repo.NewIcd10Repo(pool)
			ins, upd, err := icd.BulkUpsertSystem(ctx, codes)
			if err != nil {
				return fmt.Errorf("yükleme başarısız: %w", err)
			}
			fmt.Printf("✓ %d yeni, %d güncellendi\n", ins, upd)
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "TSV dosya yolu (varsayılan: server/data/icd10-tr-extended.tsv)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Yalnızca ayrıştır, DB'ye yazma")
	return cmd
}

// parseICD10TSV reads a TSV (tab-separated) stream and returns ICD-10 rows.
// Accepts io.Reader so the HTTP upload endpoint can reuse it.
func parseICD10TSV(r io.Reader) ([]repo.CreateIcd10Input, error) {
	out := []repo.CreateIcd10Input{}
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := strings.TrimRight(scanner.Text(), "\r\n")
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		if lineNo == 1 && strings.HasPrefix(strings.ToLower(raw), "code\t") {
			continue
		}
		parts := strings.Split(raw, "\t")
		if len(parts) < 2 {
			return nil, fmt.Errorf("satır %d: en az code + title_tr gerekli", lineNo)
		}
		c := repo.CreateIcd10Input{
			Code:    strings.TrimSpace(parts[0]),
			TitleTR: strings.TrimSpace(parts[1]),
		}
		if c.Code == "" || c.TitleTR == "" {
			return nil, fmt.Errorf("satır %d: code/title boş olamaz", lineNo)
		}
		if len(parts) >= 3 && strings.TrimSpace(parts[2]) != "" {
			chapter := strings.TrimSpace(parts[2])
			c.Chapter = &chapter
		}
		if len(parts) >= 4 && strings.TrimSpace(parts[3]) != "" {
			parent := strings.TrimSpace(parts[3])
			c.ParentCode = &parent
		}
		out = append(out, c)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
