import argparse
from pathlib import Path

from faster_whisper.utils import download_model


def main() -> None:
    parser = argparse.ArgumentParser(description="Explicitly download a faster-whisper model for offline runtime use.")
    parser.add_argument("--model", default="small")
    parser.add_argument("--output", required=True)
    parser.add_argument("--allow-download", action="store_true")
    args = parser.parse_args()
    if not args.allow_download:
        raise SystemExit("Refusing network model download without --allow-download")
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    downloaded = download_model(args.model, output_dir=str(output), local_files_only=False)
    print(downloaded)


if __name__ == "__main__":
    main()
