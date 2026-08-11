package academic

import "errors"

type Policy struct {
	MinimumPassingScoreScaled                                                      int
	ExerciseWeightBasisPoints, ExamWeightBasisPoints, MinimumAttendanceBasisPoints int
	RequireAllMandatoryExams, RequireAllMandatoryExercises                         bool
}
type Result struct {
	ExerciseAverageScaled, ExamAverageScaled, AttendanceBasisPoints, FinalScoreScaled int
	Result                                                                            string
}

func Calculate(policy Policy, exerciseAverageScaled, examAverageScaled, attendanceBasisPoints int, mandatoryComplete bool) (Result, error) {
	if policy.ExerciseWeightBasisPoints+policy.ExamWeightBasisPoints != 10000 {
		return Result{}, errors.New("assessment weights must sum to 10000")
	}
	final := (exerciseAverageScaled*policy.ExerciseWeightBasisPoints + examAverageScaled*policy.ExamWeightBasisPoints + 5000) / 10000
	outcome := "approved"
	if final < policy.MinimumPassingScoreScaled || attendanceBasisPoints < policy.MinimumAttendanceBasisPoints || !mandatoryComplete {
		outcome = "failed"
	}
	return Result{exerciseAverageScaled, examAverageScaled, attendanceBasisPoints, final, outcome}, nil
}
