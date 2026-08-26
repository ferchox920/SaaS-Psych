import argparse
from pathlib import Path

from huggingface_hub import hf_hub_download


def main() -> None:
    parser = argparse.ArgumentParser(description="Explicitly download a whisper.cpp GGML model for offline use.")
    parser.add_argument("--filename", default="ggml-small.bin")
    parser.add_argument("--output", required=True)
    parser.add_argument("--allow-download", action="store_true")
    args = parser.parse_args()
    if not args.allow_download:
        raise SystemExit("Refusing network model download without --allow-download")
    output = Path(args.output).resolve()
    output.mkdir(parents=True, exist_ok=True)
    downloaded = hf_hub_download(
        repo_id="ggerganov/whisper.cpp",
        filename=args.filename,
        local_dir=str(output),
        local_files_only=False,
    )
    print(downloaded)


if __name__ == "__main__":
    main()
