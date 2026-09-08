interface SpinnerProps {
  fullPage?: boolean;
  text?: string;
}

export function Spinner({ fullPage, text }: SpinnerProps) {
  if (fullPage) {
    return (
      <div className="spinner spinner-full">
        <span className="spinner-icon" />
        {text && <span>{text}</span>}
      </div>
    );
  }

  return (
    <span className="spinner">
      <span className="spinner-icon" />
      {text && <span>{text}</span>}
    </span>
  );
}
