import { type ReactNode } from "react";
import { Check, ChevronLeft, ChevronRight } from "lucide-react";

export interface WizardStep {
  label: string;
  content: ReactNode;
}

interface WizardProps {
  steps: WizardStep[];
  currentStep: number;
  onStepChange: (step: number) => void;
  onSubmit: () => void;
  submitLabel?: string;
  submitting?: boolean;
}

export function Wizard({
  steps,
  currentStep,
  onStepChange,
  onSubmit,
  submitLabel = "Submit",
  submitting = false,
}: WizardProps) {
  const isLast = currentStep === steps.length - 1;
  const isFirst = currentStep === 0;

  return (
    <div className="wizard">
      <WizardStepper steps={steps} currentStep={currentStep} />
      <div className="wizard-content">{steps[currentStep].content}</div>
      <div className="wizard-footer">
        <div className="wizard-footer-left">
          {!isFirst && (
            <button
              type="button"
              className="wizard-back-btn"
              onClick={() => onStepChange(currentStep - 1)}
            >
              <ChevronLeft size={16} />
              Back
            </button>
          )}
        </div>
        <div className="wizard-footer-center">
          Step {currentStep + 1} of {steps.length}
        </div>
        <div className="wizard-footer-right">
          {isLast ? (
            <button
              type="button"
              className="btn btn-primary"
              style={{ width: "auto" }}
              onClick={onSubmit}
              disabled={submitting}
            >
              {submitting ? "Submitting..." : submitLabel}
            </button>
          ) : (
            <button
              type="button"
              className="btn btn-primary"
              style={{ width: "auto" }}
              onClick={() => onStepChange(currentStep + 1)}
            >
              Next: {steps[currentStep + 1].label}
              <ChevronRight size={16} style={{ marginLeft: 4 }} />
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function WizardStepper({
  steps,
  currentStep,
}: {
  steps: WizardStep[];
  currentStep: number;
}) {
  return (
    <div className="wizard-stepper">
      {steps.map((step, i) => {
        const isCompleted = i < currentStep;
        const isActive = i === currentStep;
        const status = isCompleted ? "completed" : isActive ? "active" : "pending";

        return (
          <div key={i} className="wizard-step-group">
            {i > 0 && (
              <div
                className={`wizard-connector ${isCompleted ? "completed" : ""}`}
              />
            )}
            <div className="wizard-step-indicator-wrapper">
              <div className={`wizard-step-indicator ${status}`}>
                {isCompleted ? <Check size={14} strokeWidth={3} /> : i + 1}
              </div>
              <span className={`wizard-step-label ${status}`}>
                {step.label}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}
