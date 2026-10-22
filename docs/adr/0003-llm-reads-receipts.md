# Claude vision reads receipts

Date, total, merchant and a confidence value are extracted by the Claude API from images and PDFs; the employee checks every result before it counts. Local OCR (Tesseract) is weak on crumpled supermarket receipts. Receipt images leave the server for this; the feature can be switched off, which falls back to manual entry. When Claude declines a receipt, the API's server-side fallback answers with another model instead of failing.
