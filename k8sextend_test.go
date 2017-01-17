package k8sextend

import "testing"

func TestSampleForTest(t *testing.T) {
	// test stuff here...
	t.Log("Testing SampleForTest()")
	if SampleForTest() != 3 {
		t.Errorf("Should be 4")
	}
}
