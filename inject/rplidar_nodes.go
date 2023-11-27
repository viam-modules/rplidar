package inject

import "go.viam.com/rplidar/gen"

type Nodes struct {
	gen.Rplidar_response_measurement_node_hq_t

	SwigcptrFunc                                     func() uintptr
	SwigIsRplidar_response_measurement_node_hq_tFunc func()

	SetAngle_z_q14Func func(arg2 uint16)
	GetAngle_z_q14Func func() (_swig_ret uint16)

	SetDist_mm_q2Func func(arg2 uint)
	GetDist_mm_q2Func func() (_swig_ret uint)

	SetQualityFunc func(arg2 byte)
	GetQualityFunc func() (_swig_ret byte)

	SetFlagFunc func(arg2 byte)
	GetFlagFunc func() (_swig_ret byte)
}

func (node *Nodes) Swigcptr() uintptr {
	if node.SwigcptrFunc == nil {
		return node.Rplidar_response_measurement_node_hq_t.Swigcptr()
	}
	return node.SwigcptrFunc()
}

func (node *Nodes) SwigIsRplidar_response_measurement_node_hq_t() {
	if node.SwigIsRplidar_response_measurement_node_hq_tFunc == nil {
		node.Rplidar_response_measurement_node_hq_t.SwigIsRplidar_response_measurement_node_hq_t()
	}
	node.SwigIsRplidar_response_measurement_node_hq_tFunc()
}

func (node *Nodes) SetAngle_z_q14(arg2 uint16) {
	if node.SetAngle_z_q14Func == nil {
		node.Rplidar_response_measurement_node_hq_t.SetAngle_z_q14(arg2)
	}
	node.SetAngle_z_q14Func(arg2)
}

func (node *Nodes) GetAngle_z_q14() uint16 {
	if node.GetAngle_z_q14Func == nil {
		return node.Rplidar_response_measurement_node_hq_t.GetAngle_z_q14()
	}
	return node.GetAngle_z_q14Func()
}

func (node *Nodes) SetDist_mm_q2(arg2 uint) {
	if node.SetDist_mm_q2Func == nil {
		node.Rplidar_response_measurement_node_hq_t.SetDist_mm_q2(arg2)
	}
	node.SetDist_mm_q2Func(arg2)
}

func (node *Nodes) GetDist_mm_q2() uint {
	if node.GetDist_mm_q2Func == nil {
		return node.Rplidar_response_measurement_node_hq_t.GetDist_mm_q2()
	}
	return node.GetDist_mm_q2Func()
}

func (node *Nodes) SetQuality(arg2 byte) {
	if node.SetQualityFunc == nil {
		node.Rplidar_response_measurement_node_hq_t.SetQuality(arg2)
	}
	node.SetQualityFunc(arg2)
}

func (node *Nodes) GetQuality() byte {
	if node.GetQualityFunc == nil {
		return node.Rplidar_response_measurement_node_hq_t.GetQuality()
	}
	return node.GetQualityFunc()
}

func (node *Nodes) SetFlag(arg2 byte) {
	if node.SetFlagFunc == nil {
		node.Rplidar_response_measurement_node_hq_t.SetFlag(arg2)
	}
	node.SetFlagFunc(arg2)
}

func (node *Nodes) GetFlag() byte {
	if node.GetFlagFunc == nil {
		return node.Rplidar_response_measurement_node_hq_t.GetFlag()
	}
	return node.GetFlagFunc()
}
