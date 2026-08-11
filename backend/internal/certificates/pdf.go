package certificates

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-pdf/fpdf"
	"github.com/skip2/go-qrcode"
	"github.com/thiagomontozo/fluenthub/backend/internal/storage"
	"io"
	"strconv"
	"strings"
	"time"
)

type PDFInput struct {
	SchoolName, Title, Footer, StudentName, CourseName, LevelName, CertificateNumber, VerificationCode, VerificationBaseURL string
	Template, PrimaryColor, AccentColor                                                                                     string
	CompletionDate                                                                                                          time.Time
	WorkloadHours                                                                                                           int
	FinalScoreScaled                                                                                                        *int
}
type PDFRenderer struct{ Storage storage.ObjectStorage }

func (r PDFRenderer) Generate(ctx context.Context, input PDFInput) (string, error) {
	if err := validatePDFInput(input); err != nil {
		return "", err
	}
	qr, err := qrcode.Encode(input.VerificationBaseURL+"/certificate/verify?code="+input.VerificationCode, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("encode QR: %w", err)
	}
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(22, 18, 22)
	pdf.AddPage()
	primary := parseColor(input.PrimaryColor, [3]int{79, 70, 229})
	accent := parseColor(input.AccentColor, [3]int{245, 158, 11})
	if input.Template == "modern" {
		pdf.SetFillColor(primary[0], primary[1], primary[2])
		pdf.Rect(0, 0, 297, 34, "F")
		pdf.SetFillColor(accent[0], accent[1], accent[2])
		pdf.Rect(0, 34, 297, 3, "F")
	} else {
		pdf.SetDrawColor(primary[0], primary[1], primary[2])
		pdf.SetLineWidth(1.2)
		pdf.Rect(10, 10, 277, 190, "D")
		pdf.SetDrawColor(accent[0], accent[1], accent[2])
		pdf.SetLineWidth(0.4)
		pdf.Rect(14, 14, 269, 182, "D")
	}
	pdf.SetFont("Helvetica", "B", 14)
	if input.Template == "modern" {
		pdf.SetTextColor(255, 255, 255)
	} else {
		pdf.SetTextColor(primary[0], primary[1], primary[2])
	}
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
	pdf.CellFormat(0, 9, fmt.Sprintf("completed %s - %s", input.CourseName, input.LevelName), "", 1, "C", false, 0, "")
	pdf.Ln(5)
	details := fmt.Sprintf("Completion: %s   |   Workload: %dh", input.CompletionDate.Format("02 January 2006"), input.WorkloadHours)
	if input.FinalScoreScaled != nil {
		details += fmt.Sprintf("   |   Final score: %.2f", float64(*input.FinalScoreScaled)/100)
	}
	pdf.CellFormat(0, 9, details, "", 1, "C", false, 0, "")
	pdf.RegisterImageOptionsReader("certificate-qr", fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, bytes.NewReader(qr))
	pdf.ImageOptions("certificate-qr", 244, 145, 30, 30, false, fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, 0, "")
	pdf.SetY(160)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 6, "Certificate "+input.CertificateNumber+" | Verification "+input.VerificationCode, "", 1, "C", false, 0, "")
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

func validatePDFInput(input PDFInput) error {
	if input.Template == "" {
		input.Template = "classic"
	}
	if input.Template != "classic" && input.Template != "modern" {
		return errors.New("certificate template must be classic or modern")
	}
	if strings.TrimSpace(input.SchoolName) == "" || strings.TrimSpace(input.StudentName) == "" || strings.TrimSpace(input.CertificateNumber) == "" || len(input.VerificationCode) < 12 {
		return errors.New("certificate identity is incomplete")
	}
	if !strings.HasPrefix(input.VerificationBaseURL, "https://") && !strings.HasPrefix(input.VerificationBaseURL, "http://localhost") {
		return errors.New("verification base URL must use HTTPS")
	}
	if input.WorkloadHours < 0 || input.WorkloadHours > 100000 {
		return errors.New("invalid certificate workload")
	}
	return nil
}

func parseColor(value string, fallback [3]int) [3]int {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return fallback
	}
	result := [3]int{}
	for index := 0; index < 3; index++ {
		part, err := strconv.ParseUint(value[index*2:index*2+2], 16, 8)
		if err != nil {
			return fallback
		}
		result[index] = int(part)
	}
	return result
}
