FROM python:3.12-slim

WORKDIR /app

COPY requirements.txt .

RUN pip install --no-cache-dir -r requirements.txt

COPY . .

EXPOSE 8000

# Flask ilovasini Gunicorn yordamida ishga tushurish
CMD ["gunicorn", "--bind", "0.0.0.0:8000", "app:app"]