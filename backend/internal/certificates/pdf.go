package certificates

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
	"github.com/thiagomontozo/fluenthub/backend/internal/storage"
	"io"
	"time"
)

type PDFInput struct {
	SchoolName, Title, Footer, StudentName, CourseName, LevelName, CertificateNumber, VerificationCode, VerificationBaseURL string
	CompletionDate                                                                                                          time.Time
	WorkloadHours                                                                                                           int
	FinalScoreScaled                                                                                                        *int
}
type PDFRenderer struct{ Storage storage.ObjectStorage }

func (r PDFRenderer) Generate(ctx context.Context, input PDFInput) (string, error) {
	qr, err := qrcode.Encode(input.VerificationBaseURL+"/certificate/verify?code="+input.VerificationCode, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("encode QR: %w", err)
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(22, 18, 22)
	pdf.AddPage()
	pdf.SetDrawColor(79, 70, 229)
	pdf.SetLineWidth(1.2)
	pdf.Rect(10, 10, 277, 190, "D")
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(79, 70, 229)
	pdf.CellFormat(0, 12, input.SchoolName, "", 1, "C", false, 0, "")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "B", 30)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, 16, input.Title, "", 1, "C", false, 0, "")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 13)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 9, "This certifies that", "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "B", 25)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, 15, input.StudentName, "", 1, "C", false, 0, "")
	pdf.SetFont("Helvetica", "", 12)
	pdf.CellFormat(0, 9, fmt.Sprintf("completed %s — %s", input.CourseName, input.LevelName), "", 1, "C", false, 0, "")
	pdf.Ln(5)
	details := fmt.Sprintf("Completion: %s   •   Workload: %dh", input.CompletionDate.Format("02 January 2006"), input.WorkloadHours)
	if input.FinalScoreScaled != nil {
		details += fmt.Sprintf("   •   Final score: %.2f", float64(*input.FinalScoreScaled)/100)
	}
	pdf.CellFormat(0, 9, details, "", 1, "C", false, 0, "")
	pdf.RegisterImageOptionsReader("certificate-qr", fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, bytes.NewReader(qr))
	pdf.ImageOptions("certificate-qr", 244, 145, 30, 30, false, fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
	pdf.SetY(160)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 6, "Certificate "+input.CertificateNumber+" · Verification "+input.VerificationCode, "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 6, input.Footer, "", 1, "C", false, 0, "")
	var output bytes.Buffer
	if err := pdf.Output(&output); err != nil {
		return "", fmt.Errorf("render PDF: %w", err)
	}
	key, err := r.Storage.Put(ctx, "certificates", io.Reader(&output))
	if err != nil {
		return "", fmt.Errorf("store PDF: %w", err)
	}
	return key, nil
}
