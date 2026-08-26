import re
import unicodedata


def normalize(value: str) -> list[str]:
    value = unicodedata.normalize("NFKD", value.lower())
    value = "".join(character for character in value if not unicodedata.combining(character))
    value = re.sub(r"[^a-z0-9ñ ]+", " ", value)
    return value.split()


def word_error_rate(reference: str, hypothesis: str) -> float:
    expected, actual = normalize(reference), normalize(hypothesis)
    previous = list(range(len(actual) + 1))
    for expected_word in expected:
        current = [previous[0] + 1]
        for index, actual_word in enumerate(actual, start=1):
            current.append(min(current[-1] + 1, previous[index] + 1, previous[index - 1] + (expected_word != actual_word)))
        previous = current
    return previous[-1] / max(1, len(expected))
