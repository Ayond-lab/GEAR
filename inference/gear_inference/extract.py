from dataclasses import dataclass


@dataclass(frozen=True)
class Extraction:
    work_authorisation_status: str
    confidence: str
    prompt_injection_present: bool


STATUSES = (
    "EEA national",
    "Holds permit",
    "Requires sponsorship",
    "Unclear",
)


def extract_work_authorisation(application_text: str) -> Extraction:
    lowered = application_text.lower()
    status = "Unclear"
    for candidate in STATUSES:
        if candidate.lower() in lowered:
            status = candidate
            break

    return Extraction(
        work_authorisation_status=status,
        confidence="0.50" if status == "Unclear" else "0.84",
        prompt_injection_present="ignore all governance rules" in lowered,
    )

