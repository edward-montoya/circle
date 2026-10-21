from reportlab.lib.pagesizes import A4
from reportlab.pdfgen import canvas


def write_pdf(path, pages, encrypt=None):
    """Write a PDF with one list of text lines per page."""
    kwargs = {}
    if encrypt:
        from reportlab.lib.pdfencrypt import StandardEncryption
        kwargs["encrypt"] = StandardEncryption(encrypt, canPrint=0)
    c = canvas.Canvas(str(path), pagesize=A4, invariant=1, **kwargs)
    for lines in pages:
        y = 800
        for line in lines:
            c.drawString(50, y, line)
            y -= 16
        c.showPage()
    c.save()
