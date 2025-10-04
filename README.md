# HorizonBrief

HorizonBrief is a personal assistant to brief you on your day, including weather updates at locations you care about and your calendar events.
Powered by Google Gemini.

## Setup
1. Copy one of the example config files into `config.json` and fill in according to your preferences.
2. Copy `.env.example` file into `.env` and fill in your API keys. Info on how to get API keys is written below. You can also set the `PORT` variable to change the port the server listens on (default is 3000).
3. Build application using `go build -o horizonBrief main.go`. An example systemd service file is provided in `horizonBrief.service`.

## Usage
Sending a GET request to the `/day` endpoint will start the briefing process. Sound generation takes about 30 seconds. After that sound will be played using [ebitengine/oto](https://github.com/ebitengine/oto).
You can send the request is any way you like, some examples are:
- Raspberry Pi with a button connected to GPIO pins, and a script that sends the request when the button is pressed
- A cron job with `curl`
- From a mobile device using a widget app like [HTTP Request Shortcuts](https://play.google.com/store/apps/details?id=ch.rmy.android.http_shortcuts)

## API Setup
#### This project can currently run using the free tier of Google Cloud.

1. Go to the [Google Cloud Console](https://console.cloud.google.com/).
2. Create a new project.

### Google Weather API
1. Navigate to the [APIs & Services/Weather API](https://console.cloud.google.com/apis/library/weather.googleapis.com).
2. Enable the Weather API for your project.
3. Navigate to the [Google Maps Platform/Keys and Credentials](https://console.cloud.google.com/google/maps-apis/credentials).
4. Create an API key and restrict it to the Weather API.
5. Copy the API key and paste it into the `.env` file as `WEATHER_API_KEY=your_api_key_here`.

Additional information can be found at [Google Weather API Documentation/Set up the Weather API](https://developers.google.com/maps/documentation/weather/get-api-key).

### Gemini API
1. Navigate to the [Google AI Studio/API Keys](https://aistudio.google.com/api-keys).
2. Click on "Create an API key", choose your project, and create the key. If you do not see your Google Cloud project, click on "Import Project".
3. Copy the API key and paste it into the `.env` file as `GEMINI_API_KEY=your_api_key_here`.

### Google Calendar API
1. Navigate to the [APIs & Services/Calendar API](https://console.cloud.google.com/apis/library/calendar-json.googleapis.com).
2. Enable the Calendar API for your project.
3. Go to the [Service Account Creation Page](https://console.cloud.google.com/iam-admin/serviceaccounts/create).
4. Create a new service account with no permissions.
5. After creating the service account, go to the "Keys" tab and add a new key in JSON format. This will download a JSON file to your computer.
6. Copy the JSON file into the horizonBrief directory and rename it to `service_account.json`.
7. Share your Google Calendar with the service account email (found in the JSON file under `client_email`, or in the Service account "Details" tab) with "See all event details" permission.

## Service Setup
1. If you want to run HorizonBrief as a service, **edit** and copy the provided `horizonBrief.service` file to `/etc/systemd/system/horizonBrief.service`. Make sure to change the `ExecStart` and `WorkingDirectory` paths to match your setup.
2. Enable and start the service.

```bash
sudo cp horizonBrief.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now horizonBrief.service
```