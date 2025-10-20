# Zebrash

Library and API server for rendering ZPL (Zebra Programming Language) files as raster images

- Partially based on https://github.com/BinaryKits/BinaryKits.Zpl
- Uses slightly modified implementations of:
	- PDF417, Aztec and Code 2 of 5 barcodes from https://github.com/boombuler/barcode/
	- Code 128, Code 39, QR Code and DataMatrix from https://github.com/makiuchi-d/gozxing

## Description

This library emulates subset of ZPL engine and allows you to view most of the ZPL labels that are used by carriers such as Fedex, UPS or DHL as PNGs without the need to possess physical Zebra-compatible printer.
Think of https://labelary.com/viewer.html except it is completely free for commercial use, has no API limits and can easily be self-hosted or plugged into existing Go application so you don't need to send labels with real customers information to some 3rd-party servers

Example of the output (more examples can be found inside `testdata` folder):

![UPS label](testdata/ups.png)

## API Server

The project includes a Labelary-compatible REST API server that you can run with Docker.

### Running with Docker Compose

```bash
docker-compose up -d
```

This will start the API server on port 3030.

### Running with Docker

```bash
docker build -t zebrash-api .
docker run -p 3030:3030 zebrash-api
```

### API Usage

The API is compatible with the Labelary API interface. You can use it as a drop-in replacement by changing the hostname from `api.labelary.com` to `localhost:3030` (or your server hostname).

#### GET Request Example

```bash
curl --get http://localhost:3030/v1/printers/8dpmm/labels/4x6/0/ \
  --data-urlencode "^xa^cfa,50^fo100,100^fdHello World^fs^xz" > label.png
```

#### POST Request Example (with form data)

```bash
curl --request POST http://localhost:3030/v1/printers/8dpmm/labels/4x6/0/ \
  --data "^xa^cfa,50^fo100,100^fdHello World^fs^xz" > label.png
```

#### POST Request Example (with multipart file upload)

```bash
curl --request POST http://localhost:3030/v1/printers/8dpmm/labels/4x6/0/ \
  --form file=@label.zpl > label.png
```

#### URL Format

```
http://localhost:3030/v1/printers/{dpmm}/labels/{width}x{height}/{index}/
```

Parameters:
- `dpmm`: Dots per millimeter (e.g., 6, 8, 12, 24)
- `width`: Label width in inches (e.g., 4)
- `height`: Label height in inches (e.g., 6)
- `index`: Label index to render (0-based, use 0 for single labels)

#### Label Rotation

You can rotate the output label by adding the `X-Rotation` header to your request:

```bash
curl --request POST http://localhost:3030/v1/printers/8dpmm/labels/4x6/0/ \
  --header "X-Rotation: 90" \
  --data "^xa^cfa,50^fo100,100^fdHello World^fs^xz" > label.png
```

Supported rotation values: `0`, `90`, `180`, `270` (degrees clockwise)

### Health Check

```bash
curl http://localhost:3030/health
```

## Library Usage

You can also use Zebrash as a Go library in your own applications:

```go

	file, err := os.ReadFile("./testdata/label.zpl")
	if err != nil {
		t.Fatal(err)
	}

	parser := zebrash.NewParser()

	res, err := parser.Parse(file)
	if err != nil {
		t.Fatal(err)
	}

	var buff bytes.Buffer

	drawer := zebrash.NewDrawer()

	err = drawer.DrawLabelAsPng(res[0], &buff, drawers.DrawerOptions{
		LabelWidthMm:  101.6,
		LabelHeightMm: 203.2,
		Dpmm:          8,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile("./testdata/label.png", buff.Bytes(), 0744)
	if err != nil {
		t.Fatal(err)
	}

```