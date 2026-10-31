# 100% pure 7-bit ASCII
from fastapi import APIRouter
from pydantic import BaseModel, Field

from ai.pipelines.document.resume_parser import ParsedStudentProfile, ResumeParser

router = APIRouter(prefix="/internal/v1/resumes", tags=["resumes"])
parser = ResumeParser()


class ResumeParseRequest(BaseModel):
    student_id: str = Field(default="")
    text: str


@router.post("/parse", response_model=ParsedStudentProfile)
async def parse_resume_direct(payload: ResumeParseRequest):
    return parser.extract_profile_from_text(payload.text)


@router.post("/parse-async")
async def parse_resume_async(payload: ResumeParseRequest):
    profile = parser.extract_profile_from_text(payload.text)
    return {
        "job_id": f"sync-{payload.student_id}",
        "status": "completed",
        "profile": profile,
    }
