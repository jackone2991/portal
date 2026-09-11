# Comic scraper service (SPEC-02 P1.8): FastAPI + SeleniumBase (undetected Chrome)
# on a virtual display (xvfb). Headful-on-xvfb is far less likely to be flagged by
# Cloudflare than headless-in-Docker — but if a target still blocks the container,
# run this service on the host instead (see README) and point COMIC_SCRAPER_URL at it.
FROM python:3.12-slim

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    SE_AVOID_STATS=true

# Google Chrome + xvfb + the shared libs Chrome needs to run headful.
RUN apt-get update && apt-get install -y --no-install-recommends \
      wget gnupg ca-certificates unzip xvfb \
      fonts-liberation libnss3 libxss1 libasound2 libatk-bridge2.0-0 libatk1.0-0 \
      libcups2 libdrm2 libgbm1 libgtk-3-0 libx11-xcb1 libxcomposite1 libxdamage1 \
      libxrandr2 libxfixes3 xdg-utils \
 && wget -q -O /tmp/chrome.deb https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb \
 && apt-get install -y --no-install-recommends /tmp/chrome.deb \
 && rm -f /tmp/chrome.deb \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .

EXPOSE 8000
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8000"]
