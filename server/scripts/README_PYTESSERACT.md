## PyTesseract OCR Fallback Setup

The screenshot parser route now supports a local OCR fallback using `pytesseract`:

- Primary: Claude vision API
- Fallback 1: local Python OCR (`pytesseract`)
- Fallback 2: deterministic demo response

### Install dependencies (local machine)

1. Install Tesseract binary:
   - macOS (Homebrew): `brew install tesseract`

2. Install Python deps:
   - `python3 -m pip install pytesseract pillow requests`

### Notes

- OCR fallback script path: `server/scripts/screenshot_ocr_fallback.py`
- Route using it: `POST /api/screenshot/parse`
- OCR output is heuristic and lower-confidence than Claude; users should review slots.
