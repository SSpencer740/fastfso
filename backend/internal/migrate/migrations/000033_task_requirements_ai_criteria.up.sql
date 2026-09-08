-- AI verification criteria for file_upload requirements. When non-null/non-empty,
-- uploads to this requirement are restricted to AI-supported content types
-- (PDF, PNG, JPG, WEBP) and trigger an asynchronous Gemini verification job.
-- Null/empty means no AI verification — any file type allowed, no AI runs.
ALTER TABLE task_requirements
    ADD COLUMN ai_verification_criteria TEXT NOT NULL DEFAULT '';
